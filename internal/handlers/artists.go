package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"palasgroupietracker/internal/api"
)

type MemberFilterOption struct {
	Value    int
	Label    string
	Selected bool
}

type ArtistsPageData struct {
	Title         string
	Artists       []api.Artist
	Query         string
	YearMin       string
	YearMax       string
	MemberOptions []MemberFilterOption
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
	memberParams := r.URL.Query()["members"]

	yearMin, _ := strconv.Atoi(yearMinStr)
	yearMax, _ := strconv.Atoi(yearMaxStr)

	selectedMemberCounts := make(map[int]bool)
	for _, v := range memberParams {
		if n, err := strconv.Atoi(v); err == nil {
			selectedMemberCounts[n] = true
		}
	}

	baseCounts := []int{1, 2, 3, 4, 5, 6, 7, 8}
	memberOptions := make([]MemberFilterOption, 0, len(baseCounts))
	for _, c := range baseCounts {
		label := fmt.Sprintf("%d members", c)
		if c == 1 {
			label = "1 member"
		}
		_, selected := selectedMemberCounts[c]
		memberOptions = append(memberOptions, MemberFilterOption{
			Value:    c,
			Label:    label,
			Selected: selected,
		})
	}

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
		if len(selectedMemberCounts) > 0 && !selectedMemberCounts[memberCount] {
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
		Title:         "Artists",
		Artists:       filtered,
		Query:         query,
		YearMin:       yearMinStr,
		YearMax:       yearMaxStr,
		MemberOptions: memberOptions,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}
