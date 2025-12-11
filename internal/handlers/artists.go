package handlers

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"palasgroupietracker/internal/api"
)

type SpotifyArtistView struct {
	Artist           api.SpotifyArtist
	Followers        int
	MonthlyListeners int
}

type ArtistsPageData struct {
	Title      string
	Source     string
	Artists    []api.Artist
	Spotify    []SpotifyArtistView
	Query      string
	YearMin    string
	YearMax    string
	MembersMin string
	MembersMax string
	Sort       string
}

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)

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
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func ArtistsAjaxHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)

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
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "artist_list", data); err != nil {
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
		Sort:       "",
	}

	return data, nil
}

func buildSpotifyData(r *http.Request) (ArtistsPageData, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))

	if query == "" {
		query = "a"
	}

	results, err := api.SearchSpotifyArtists(query)
	if err != nil {
		return ArtistsPageData{}, err
	}

	views := make([]SpotifyArtistView, len(results))
	for i, a := range results {
		views[i].Artist = a
		if a.Followers != nil {
			views[i].Followers = a.Followers.Total
		}
	}

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i := range views {
		wg.Add(1)
		go func(v *SpotifyArtistView) {
			defer wg.Done()
			sem <- struct{}{}
			listeners, err := api.FetchArtistMonthlyListeners(v.Artist.Name)
			if err != nil {
				listeners = 0
			}
			v.MonthlyListeners = listeners
			<-sem
		}(&views[i])
	}

	wg.Wait()

	isDefaultQuery := strings.TrimSpace(r.URL.Query().Get("q")) == ""

	if isDefaultQuery {
		if sortParam == "" {
			sortParam = "relevance"
		}
	} else {
		if sortParam == "" {
			sortParam = "relevance"
		}
	}

	switch sortParam {
	case "followers_asc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Followers < views[j].Followers
		})
	case "followers_desc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Followers > views[j].Followers
		})
	case "listeners_asc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].MonthlyListeners < views[j].MonthlyListeners
		})
	case "listeners_desc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].MonthlyListeners > views[j].MonthlyListeners
		})
	case "relevance":
	default:
		sortParam = "relevance"
	}

	data := ArtistsPageData{
		Title:   "Artists",
		Source:  "spotify",
		Spotify: views,
		Query:   strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:    sortParam,
	}

	return data, nil
}
