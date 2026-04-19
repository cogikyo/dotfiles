// Package main implements newtab, the HTTP backend for the Firefox custom new-tab page.
//
// Responsibilities:
// - Serve static assets and JSON endpoints for the page.
// - Query Firefox places.sqlite for bookmarks and history.
// - Proxy suggestion requests to Google's Firefox endpoint.
package main

// main.go defines server startup, Firefox DB discovery, and API handlers.
import (
	"bufio"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ constants                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

const (
	defaultPort         = ":42069"
	defaultStaticDir    = "dotfiles/daemons/newtab"
	defaultHistoryLimit = 15

	// Google suggest endpoint (Firefox format); returns [query, [suggestions...]].
	googleSuggest = "https://suggestqueries.google.com/complete/search?client=firefox&q="
)

// SQL fragments shared by handleHistory so both branches filter identically.
const (
	urlFilter   = "p.url NOT LIKE 'about:%' AND p.url NOT LIKE 'moz-%'"
	titleFilter = "p.title IS NOT NULL AND p.title != ''"
	visitFilter = "p.visit_count > 0"
)

var homeDir = os.Getenv("HOME")

var suggestClient = &http.Client{Timeout: 5 * time.Second}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ config                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

type newtabConfig struct {
	Port         string
	FirefoxDB    string
	StaticDir    string
	HistoryLimit int
}

// resolveFirefoxDB finds places.sqlite by scanning Firefox profile roots (~/.mozilla/firefox, ~/.config/mozilla/firefox), preferring dev-edition-default.
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

// dbPath returns a SQLite DSN with mode=ro&immutable=1 so we can query while Firefox holds a write lock.
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

// Bookmark is a moz_bookmarks row joined with its place, folder, keyword, and tags.
//
// Pointer fields serialize as JSON null when absent.
type Bookmark struct {
	Folder  string  `json:"folder"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Keyword *string `json:"keyword"`
	Tags    *string `json:"tags"`
}

// HistoryEntry is a row from moz_places.
//
// LastVisit is unix seconds (Firefox stores microseconds; divided at query time).
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

func openPlacesDB(cfg newtabConfig) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath(cfg))
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ handlers                                                                     │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// handleBookmarks returns all bookmarks grouped by folder with keyword and tag metadata.
//
// Firefox tags are bookmarks whose parent is the top-level "tags" folder; these are aggregated via subquery and the folder itself is excluded.
func handleBookmarks(cfg newtabConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := openPlacesDB(cfg)
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
		if err := rows.Err(); err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, bookmarks)
	}
}

// handleHistory returns recent or query-matched places from moz_places.
//
// Without ?q=: most-recently-visited entries.
// With ?q=: case-insensitive substring match on title/URL, ranked by visit_count blended with recency (1e12 divisor rescales microsecond timestamps to visit-count magnitude).
func handleHistory(cfg newtabConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := openPlacesDB(cfg)
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
		if err := rows.Err(); err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, history)
	}
}

// handleSuggest proxies Google suggest and returns a flat JSON string array.
//
// Upstream shape is [query, [suggestions...], ...]; only index 1 is extracted.
// Failures return [] so the frontend degrades cleanly.
func handleSuggest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, []string{})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, googleSuggest+url.QueryEscape(query), nil)
	if err != nil {
		writeJSON(w, []string{})
		return
	}

	resp, err := suggestClient.Do(req)
	if err != nil {
		writeJSON(w, []string{})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, []string{})
		return
	}

	var result []any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result) < 2 {
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
