package handlers

import "html/template"

// templateWithLayout parses the shared layout together with a page template.
func templateWithLayout(page string) (*template.Template, error) {
	return template.ParseFiles("web/templates/layout.gohtml", page)
}
