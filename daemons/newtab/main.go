package main

import (
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

// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

const (
	port          = ":42069"
	firefoxDB     = ".mozilla/firefox/sdfm8kqz.dev-edition-default/places.sqlite"
	staticDir     = "dotfiles/daemons/newtab"
	googleSuggest = "https://suggestqueries.google.com/complete/search?client=firefox&q="
	historyLimit  = 15
)

const (
	urlFilter   = "p.url NOT LIKE 'about:%' AND p.url NOT LIKE 'moz-%'"
	titleFilter = "p.title IS NOT NULL AND p.title != ''"
	visitFilter = "p.visit_count > 0"
)

var homeDir = os.Getenv("HOME")

func dbPath() string {
	return "file:" + filepath.Join(homeDir, firefoxDB) + "?mode=ro&immutable=1"
}

func staticPath() string {
	return filepath.Join(homeDir, staticDir)
}

// ═══════════════════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════════════════

type Bookmark struct {
	Folder  string  `json:"folder"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Keyword *string `json:"keyword"`
	Tags    *string `json:"tags"`
}

type HistoryEntry struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	VisitCount int    `json:"visit_count"`
	LastVisit  int64  `json:"last_visit"`
}

// ═══════════════════════════════════════════════════════════════════════════
// Main
// ═══════════════════════════════════════════════════════════════════════════

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/bookmarks", withCORS(handleBookmarks))
	mux.HandleFunc("/api/history", withCORS(handleHistory))
	mux.HandleFunc("/api/suggest", withCORS(handleSuggest))
	mux.Handle("/", http.FileServer(http.Dir(staticPath())))

	log.Printf("newtab-server listening on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

// ═══════════════════════════════════════════════════════════════════════════
// Middleware
// ═══════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════
// Handlers
// ═══════════════════════════════════════════════════════════════════════════

func handleBookmarks(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite", dbPath())
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

func handleHistory(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite", dbPath())
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
		`, historyLimit)
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
		`, searchTerm, searchTerm, historyLimit)
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
