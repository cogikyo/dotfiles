// Package main implements newtab, an HTTP service backing the custom new-tab page.
//
// It queries Firefox's places.sqlite for history and bookmarks, proxies Google suggest, and serves JSON.
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ constants                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

const (
	defaultPort         = ":42069"
	defaultStaticDir    = "dotfiles/daemons/newtab"
	defaultHistoryLimit = 15

	// Google's Firefox-format suggest endpoint. Returns [query, [suggestions...]].
	googleSuggest = "https://suggestqueries.google.com/complete/search?client=firefox&q="
)

// SQL fragments shared by both branches of handleHistory so filtering stays identical.
const (
	// Exclude internal Firefox URLs (about:config, moz-extension://, etc).
	urlFilter = "p.url NOT LIKE 'about:%' AND p.url NOT LIKE 'moz-%'"

	// Drop rows with no usable title — they render as blank suggestions.
	titleFilter = "p.title IS NOT NULL AND p.title != ''"

	// Exclude places inserted by bookmarks but never visited.
	visitFilter = "p.visit_count > 0"
)

var homeDir = os.Getenv("HOME")

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ config                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

type newtabConfig struct {
	Port         string
	FirefoxDB    string
	StaticDir    string
	HistoryLimit int
}

// resolveFirefoxDB returns an absolute path to places.sqlite by scanning both
// Firefox profile roots (legacy ~/.mozilla/firefox and XDG ~/.config/mozilla/firefox),
// preferring a profile named dev-edition-default, then any profile marked Default=1.
func resolveFirefoxDB() string {
	roots := []string{
		filepath.Join(homeDir, ".mozilla", "firefox"),
		filepath.Join(homeDir, ".config", "mozilla", "firefox"),
	}

	var fallback string
	for _, root := range roots {
		iniPath := filepath.Join(root, "profiles.ini")
		f, err := os.Open(iniPath)
		if err != nil {
			continue
		}
		section, name, path := "", "", ""
		scanner := bufio.NewScanner(f)
		flush := func() {
			if !strings.HasPrefix(section, "Profile") || path == "" {
				return
			}
			db := filepath.Join(root, path, "places.sqlite")
			if _, err := os.Stat(db); err != nil {
				return
			}
			if name == "dev-edition-default" {
				fallback = db
				return
			}
			if fallback == "" {
				fallback = db
			}
		}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				flush()
				section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
				name, path = "", ""
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "Name":
				name = strings.TrimSpace(value)
			case "Path":
				path = strings.TrimSpace(value)
			}
		}
		flush()
		f.Close()
		if fallback != "" && strings.Contains(fallback, "dev-edition-default") {
			return fallback
		}
	}
	return fallback
}

// dbPath returns a SQLite DSN opening places.sqlite read-only and immutable.
//
// immutable=1 lets us query while Firefox holds a write lock on the live DB.
// cfg.FirefoxDB is expected to be absolute (main() resolves it at startup).
func dbPath(cfg newtabConfig) string {
	p := cfg.FirefoxDB
	if !filepath.IsAbs(p) {
		p = filepath.Join(homeDir, p)
	}
	return "file:" + p + "?mode=ro&immutable=1"
}

func staticPath(cfg newtabConfig) string {
	return filepath.Join(homeDir, cfg.StaticDir)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ types                                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Bookmark is one moz_bookmarks row joined with its place, folder, keyword, and aggregated tags.
//
// Keyword and Tags are pointers so absent values serialize as JSON null rather than empty string.
type Bookmark struct {
	Folder  string  `json:"folder"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Keyword *string `json:"keyword"`
	Tags    *string `json:"tags"`
}

// HistoryEntry is one row from moz_places.
//
// LastVisit is unix seconds, converted from Firefox's microsecond-precision last_visit_date.
type HistoryEntry struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	VisitCount int    `json:"visit_count"`
	LastVisit  int64  `json:"last_visit"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ main                                                                         │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func main() {
	cfg := newtabConfig{
		Port:         defaultPort,
		StaticDir:    defaultStaticDir,
		HistoryLimit: defaultHistoryLimit,
		FirefoxDB:    resolveFirefoxDB(),
	}
	if cfg.FirefoxDB == "" {
		log.Print("warning: no Firefox places.sqlite found; bookmarks/history queries will fail until Firefox is run")
	} else {
		log.Printf("firefox places.sqlite: %s", cfg.FirefoxDB)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/bookmarks", withCORS(handleBookmarks(cfg)))
	mux.HandleFunc("/api/history", withCORS(handleHistory(cfg)))
	mux.HandleFunc("/api/suggest", withCORS(handleSuggest))
	mux.Handle("/", http.FileServer(http.Dir(staticPath(cfg))))

	log.Printf("newtab-server listening on http://localhost%s", cfg.Port)
	log.Fatal(http.ListenAndServe(cfg.Port, mux))
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ middleware                                                                   │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// withCORS wraps next with permissive CORS and a JSON content type.
//
// The frontend is served from the same process, so wildcard origin is fine.
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ handlers                                                                     │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// handleBookmarks returns all bookmarks grouped by folder, with keyword and tag metadata attached.
//
// Firefox's tag model: tags are bookmarks whose parent is the top-level 'tags' folder, linked via moz_bookmarks.fk.
// The 'tags' folder itself is filtered out so tag entries don't surface as bookmarks.
func handleBookmarks(cfg newtabConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("sqlite", dbPath(cfg))
		if err != nil {
			writeError(w, err)
			return
		}
		defer db.Close()

		rows, err := db.Query(`
		SELECT
			COALESCE(f.title, 'unsorted') as folder,
			b.title as title,
			p.url as url,
			k.keyword as keyword,
			(
				SELECT GROUP_CONCAT(t.title)
				FROM moz_bookmarks tag_link
				JOIN moz_bookmarks t ON tag_link.parent = t.id
				WHERE tag_link.fk = p.id
				AND t.parent = (SELECT id FROM moz_bookmarks WHERE title = 'tags' AND parent = 1)
			) as tags
		FROM moz_bookmarks b
		JOIN moz_places p ON b.fk = p.id
		LEFT JOIN moz_bookmarks f ON b.parent = f.id
		LEFT JOIN moz_keywords k ON p.id = k.place_id
		WHERE p.url NOT LIKE 'place:%'
			AND b.title IS NOT NULL
			AND b.title != ''
			AND (f.title IS NULL OR f.title != 'tags')
		ORDER BY f.title, b.position
	`)
		if err != nil {
			writeError(w, err)
			return
		}
		defer rows.Close()

		bookmarks := []Bookmark{}
		for rows.Next() {
			var b Bookmark
			if err := rows.Scan(&b.Folder, &b.Title, &b.URL, &b.Keyword, &b.Tags); err != nil {
				continue
			}
			bookmarks = append(bookmarks, b)
		}

		writeJSON(w, bookmarks)
	}
}

// handleHistory returns recent or query-matched places from moz_places.
//
// No ?q=: most-recently-visited entries.
// With ?q=: case-insensitive substring match on title/URL, ranked by visit_count blended with recency.
// The 1e12 divisor rescales microsecond timestamps to the same order of magnitude as visit counts.
func handleHistory(cfg newtabConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("sqlite", dbPath(cfg))
		if err != nil {
			writeError(w, err)
			return
		}
		defer db.Close()

		query := r.URL.Query().Get("q")
		var rows *sql.Rows

		if query == "" {
			rows, err = db.Query(`
			SELECT
				COALESCE(p.title, '') as title,
				p.url,
				p.visit_count,
				p.last_visit_date / 1000000 as last_visit
			FROM moz_places p
			WHERE `+visitFilter+`
				AND `+titleFilter+`
				AND `+urlFilter+`
			ORDER BY p.last_visit_date DESC
			LIMIT ?
		`, cfg.HistoryLimit)
		} else {
			searchTerm := "%" + strings.ToLower(query) + "%"
			rows, err = db.Query(`
			SELECT
				COALESCE(p.title, '') as title,
				p.url,
				p.visit_count,
				p.last_visit_date / 1000000 as last_visit
			FROM moz_places p
			WHERE `+visitFilter+`
				AND `+titleFilter+`
				AND `+urlFilter+`
				AND (LOWER(p.title) LIKE ? OR LOWER(p.url) LIKE ?)
			ORDER BY
				p.visit_count * 0.3 + (p.last_visit_date / 1000000000000.0) DESC
			LIMIT ?
		`, searchTerm, searchTerm, cfg.HistoryLimit)
		}

		if err != nil {
			writeError(w, err)
			return
		}
		defer rows.Close()

		history := []HistoryEntry{}
		for rows.Next() {
			var h HistoryEntry
			if err := rows.Scan(&h.Title, &h.URL, &h.VisitCount, &h.LastVisit); err != nil {
				continue
			}
			history = append(history, h)
		}

		writeJSON(w, history)
	}
}

// handleSuggest proxies Google's Firefox-format suggest API as a flat JSON array of strings.
//
// Upstream shape is [query, [suggestions...], ...]; only index 1 is used.
// Upstream or decode failures return [] instead of an error so the frontend renders cleanly.
func handleSuggest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, []string{})
		return
	}

	resp, err := http.Get(googleSuggest + url.QueryEscape(query))
	if err != nil {
		writeJSON(w, []string{})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, []string{})
		return
	}

	var result []any
	if err := json.Unmarshal(body, &result); err != nil || len(result) < 2 {
		writeJSON(w, []string{})
		return
	}

	suggestions, ok := result[1].([]any)
	if !ok {
		writeJSON(w, []string{})
		return
	}

	out := []string{}
	for _, s := range suggestions {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}

	writeJSON(w, out)
}
