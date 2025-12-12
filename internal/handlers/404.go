package handlers

import (
	"html/template"
	"net/http"
)

type NotFoundPageData struct {
	Title     string
	Source    string
	ActiveNav string
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/404.gohtml",
	)
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	data := NotFoundPageData{
		Title:     "Page not found",
		Source:    getSource(r),
		ActiveNav: "",
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}
}
