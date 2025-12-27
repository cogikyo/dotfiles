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

const (
	port          = ":42069"
	firefoxDB     = ".mozilla/firefox/sdfm8kqz.dev-edition-default/places.sqlite"
	staticDir     = "dotfiles/share/newtab"
	googleSuggest = "https://suggestqueries.google.com/complete/search?client=firefox&q="
)

var homeDir string

func init() {
	homeDir, _ = os.UserHomeDir()
}

func dbPath() string {
	return "file:" + filepath.Join(homeDir, firefoxDB) + "?mode=ro&immutable=1"
}

func staticPath() string {
	return filepath.Join(homeDir, staticDir)
}

// Bookmark represents a Firefox bookmark
type Bookmark struct {
	Folder  string  `json:"folder"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Keyword *string `json:"keyword"`
	Tags    *string `json:"tags"`
}

// HistoryEntry represents a Firefox history entry
type HistoryEntry struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	VisitCount int    `json:"visit_count"`
	LastVisit  int64  `json:"last_visit"`
}

func main() {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/bookmarks", corsMiddleware(handleBookmarks))
	mux.HandleFunc("/api/history", corsMiddleware(handleHistory))
	mux.HandleFunc("/api/suggest", corsMiddleware(handleSuggest))

	// Static files
	fs := http.FileServer(http.Dir(staticPath()))
	mux.Handle("/", fs)

	log.Printf("newtab-server listening on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func handleBookmarks(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite", dbPath())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	json.NewEncoder(w).Encode(bookmarks)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit := 15

	db, err := sql.Open("sqlite", dbPath())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var rows *sql.Rows

	if query == "" {
		// Return recent history
		rows, err = db.Query(`
			SELECT 
				COALESCE(p.title, '') as title,
				p.url,
				p.visit_count,
				p.last_visit_date / 1000000 as last_visit
			FROM moz_places p
			WHERE p.visit_count > 0
				AND p.title IS NOT NULL
				AND p.title != ''
				AND p.url NOT LIKE 'about:%'
				AND p.url NOT LIKE 'moz-%'
			ORDER BY p.last_visit_date DESC
			LIMIT ?
		`, limit)
	} else {
		// Search history by title/URL
		searchTerm := "%" + strings.ToLower(query) + "%"
		rows, err = db.Query(`
			SELECT 
				COALESCE(p.title, '') as title,
				p.url,
				p.visit_count,
				p.last_visit_date / 1000000 as last_visit
			FROM moz_places p
			WHERE p.visit_count > 0
				AND p.title IS NOT NULL
				AND p.title != ''
				AND p.url NOT LIKE 'about:%'
				AND p.url NOT LIKE 'moz-%'
				AND (LOWER(p.title) LIKE ? OR LOWER(p.url) LIKE ?)
			ORDER BY 
				p.visit_count * 0.3 + (p.last_visit_date / 1000000000000.0) DESC
			LIMIT ?
		`, searchTerm, searchTerm, limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	json.NewEncoder(w).Encode(history)
}

func handleSuggest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	resp, err := http.Get(googleSuggest + url.QueryEscape(query))
	if err != nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	// Google returns: ["query", ["suggestion1", "suggestion2", ...]]
	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil || len(result) < 2 {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	suggestions, ok := result[1].([]interface{})
	if !ok {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	out := []string{}
	for _, s := range suggestions {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}

	json.NewEncoder(w).Encode(out)
}
