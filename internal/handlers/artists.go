package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"palasgroupietracker/internal/api"
)

type ArtistsPageData struct {
	Title      string
	Source     string
	Artists    []api.Artist
	Spotify    []api.SpotifyArtist
	Query      string
	YearMin    string
	YearMax    string
	MembersMin string
	MembersMax string
}

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	log.Printf("artists: page request, source=%s, rawQuery=%q\n", source, r.URL.RawQuery)

	var data ArtistsPageData
	var err error

	if source == "spotify" {
		data, err = buildSpotifyData(r)
	} else {
		data, err = buildGroupieData(r)
	}

	if err != nil {
		http.Error(w, "failed to load artists", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artists.gohtml",
	)
	if err != nil {
		log.Printf("artists: template error (page): %v\n", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("artists: render error (page): %v\n", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func ArtistsAjaxHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	log.Printf("artists: ajax request, source=%s, rawQuery=%q\n", source, r.URL.RawQuery)

	var data ArtistsPageData
	var err error

	if source == "spotify" {
		data, err = buildSpotifyData(r)
	} else {
		data, err = buildGroupieData(r)
	}

	if err != nil {
		http.Error(w, "failed to load artists", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("web/templates/artists.gohtml")
	if err != nil {
		log.Printf("artists: template error (ajax): %v\n", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "artist_list", data)
	if err != nil {
		log.Printf("artists: render error (ajax): %v\n", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func buildGroupieData(r *http.Request) (ArtistsPageData, error) {
	artists, err := api.FetchArtists()
	if err != nil {
		return ArtistsPageData{}, err
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

	data := ArtistsPageData{
		Title:      "Artists",
		Source:     "groupie",
		Artists:    filtered,
		Query:      query,
		YearMin:    yearMinStr,
		YearMax:    yearMaxStr,
		MembersMin: membersMinStr,
		MembersMax: membersMaxStr,
	}

	return data, nil
}

func buildSpotifyData(r *http.Request) (ArtistsPageData, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	results, err := api.SearchSpotifyArtists(query)
	if err != nil {
		return ArtistsPageData{}, err
	}

	data := ArtistsPageData{
		Title:   "Artists",
		Source:  "spotify",
		Spotify: results,
		Query:   query,
	}

	return data, nil
}
