package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"palasgroupietracker/internal/api"
)

type ArtistsPageData struct {
	Title      string
	Artists    []api.Artist
	Query      string
	YearMin    string
	YearMax    string
	MembersMin string
	MembersMax string
}

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	artists, err := api.FetchArtists()
	if err != nil {
		http.Error(w, "failed to load artists", http.StatusInternalServerError)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	yearMinStr := strings.TrimSpace(r.URL.Query().Get("year_min"))
	yearMaxStr := strings.TrimSpace(r.URL.Query().Get("year_max"))
	membersMinStr := strings.TrimSpace(r.URL.Query().Get("members_min"))
	membersMaxStr := strings.TrimSpace(r.URL.Query().Get("members_max"))

	yearMin, _ := strconv.Atoi(yearMinStr)
	yearMax, _ := strconv.Atoi(yearMaxStr)
	membersMin, _ := strconv.Atoi(membersMinStr)
	membersMax, _ := strconv.Atoi(membersMaxStr)

	filtered := make([]api.Artist, 0, len(artists))
	lowerQuery := strings.ToLower(query)

	for _, a := range artists {
		if lowerQuery != "" {
			matched := strings.Contains(strings.ToLower(a.Name), lowerQuery)
			if !matched {
				for _, m := range a.Members {
					if strings.Contains(strings.ToLower(m), lowerQuery) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		if yearMin > 0 && a.CreationDate < yearMin {
			continue
		}
		if yearMax > 0 && a.CreationDate > yearMax {
			continue
		}

		memberCount := len(a.Members)
		if membersMin > 0 && memberCount < membersMin {
			continue
		}
		if membersMax > 0 && memberCount > membersMax {
			continue
		}

		filtered = append(filtered, a)
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
		Title:      "Artists",
		Artists:    filtered,
		Query:      query,
		YearMin:    yearMinStr,
		YearMax:    yearMaxStr,
		MembersMin: membersMinStr,
		MembersMax: membersMaxStr,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}
