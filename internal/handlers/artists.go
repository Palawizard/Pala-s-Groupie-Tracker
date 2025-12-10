package handlers

import (
	"html/template"
	"net/http"

	"palasgroupietracker/internal/api"
)

type ArtistsPageData struct {
	Title   string
	Artists []api.Artist
}

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	artists, err := api.FetchArtists()
	if err != nil {
		http.Error(w, "failed to load artists", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artists.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistsPageData{
		Title:   "Artists",
		Artists: artists,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}
