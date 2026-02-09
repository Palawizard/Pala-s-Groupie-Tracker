package handlers

import (
	"net/http"
	"strings"
)

// normalizeSource validates a requested source and falls back to groupie
func normalizeSource(source string) string {
	s := strings.TrimSpace(strings.ToLower(source))
	switch s {
	case "spotify", "deezer", "apple", "groupie":
		return s
	default:
		return "groupie"
	}
}

// getSource reads the `source` query parameter and returns a safe known value
func getSource(r *http.Request) string {
	return normalizeSource(r.URL.Query().Get("source"))
}
