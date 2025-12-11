package handlers

import (
	"html/template"
	"net/http"
)

type HomePageData struct {
	Title  string
	Source string
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/home.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := HomePageData{
		Title:  "Groupie Tracker",
		Source: source,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}
