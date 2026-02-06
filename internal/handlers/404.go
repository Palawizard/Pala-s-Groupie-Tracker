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

// NotFound renders the custom 404 page using the shared layout
func NotFound(w http.ResponseWriter, r *http.Request) {
	// Set status before writing any body content
	w.WriteHeader(http.StatusNotFound)

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/404.gohtml",
	)
	if err != nil {
		// Fall back to a plain message if templates are broken
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
