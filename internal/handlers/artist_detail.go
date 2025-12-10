package handlers

import (
	"html/template"
	"net/http"
	"path"
	"strconv"

	"palasgroupietracker/internal/api"
)

type ArtistDetailPageData struct {
	Title    string
	Artist   *api.Artist
	Relation *api.Relation
}

func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := path.Base(r.URL.Path)
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	artist, err := api.FetchArtistByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	relation, err := api.FetchRelationForArtist(id)
	if err != nil {
		http.Error(w, "failed to load concerts", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:    artist.Name,
		Artist:   artist,
		Relation: relation,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}
