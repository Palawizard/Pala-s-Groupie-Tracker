package handlers

import "net/http"

func getSource(r *http.Request) string {
	source := r.URL.Query().Get("source")
	if source == "spotify" {
		return "spotify"
	}
	if source == "deezer" {
		return "deezer"
	}
	return "groupie"
}
