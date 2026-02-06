package handlers

import "net/http"

// getSource reads the `source` query parameter and returns a safe known value
func getSource(r *http.Request) string {
	source := r.URL.Query().Get("source")
	if source == "spotify" {
		return "spotify"
	}
	if source == "deezer" {
		return "deezer"
	}
	if source == "apple" {
		return "apple"
	}
	// Default to the original Groupie Tracker dataset
	return "groupie"
}
