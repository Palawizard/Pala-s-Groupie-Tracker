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

type DeezerArtistView struct {
	Artist   api.DeezerArtist
	Fans     int
	Albums   int
	HasRadio bool
}

type AppleArtistView struct {
	Artist   api.AppleArtist
	ImageURL string
	Genre    string
}

type ArtistsPageData struct {
	Title           string
	Source          string
	Artists         []api.Artist
	Spotify         []SpotifyArtistView
	Deezer          []DeezerArtistView
	Apple           []AppleArtistView
	Query           string
	YearMin         string
	YearMax         string
	MembersMin      string
	MembersMax      string
	Sort            string
	ActiveNav       string
	YearMinBound    int
	YearMaxBound    int
	MembersMinBound int
	MembersMaxBound int
	YearMinValue    int
	YearMaxValue    int
	MembersMinValue int
	MembersMaxValue int
}

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)

	var data ArtistsPageData
	var err error

	if source == "spotify" {
		data, err = buildSpotifyData(r)
	} else if source == "deezer" {
		data, err = buildDeezerData(r)
	} else if source == "apple" {
		data, err = buildAppleData(r)
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
	} else if source == "deezer" {
		data, err = buildDeezerData(r)
	} else if source == "apple" {
		data, err = buildAppleData(r)
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

	yearMinBound, yearMaxBound, membersMinBound, membersMaxBound := computeGroupieBounds(artists)

	yearMin, _ := strconv.Atoi(yearMinStr)
	yearMax, _ := strconv.Atoi(yearMaxStr)
	membersMin, _ := strconv.Atoi(membersMinStr)
	membersMax, _ := strconv.Atoi(membersMaxStr)

	yearMinValue := yearMin
	yearMaxValue := yearMax
	membersMinValue := membersMin
	membersMaxValue := membersMax

	if yearMinValue == 0 {
		yearMinValue = yearMinBound
	}
	if yearMaxValue == 0 {
		yearMaxValue = yearMaxBound
	}
	if membersMinValue == 0 {
		membersMinValue = membersMinBound
	}
	if membersMaxValue == 0 {
		membersMaxValue = membersMaxBound
	}

	if yearMinValue < yearMinBound {
		yearMinValue = yearMinBound
	}
	if yearMaxValue > yearMaxBound {
		yearMaxValue = yearMaxBound
	}
	if membersMinValue < membersMinBound {
		membersMinValue = membersMinBound
	}
	if membersMaxValue > membersMaxBound {
		membersMaxValue = membersMaxBound
	}

	if yearMinValue > yearMaxValue {
		yearMinValue = yearMaxValue
	}
	if membersMinValue > membersMaxValue {
		membersMinValue = membersMaxValue
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

		if yearMinValue > yearMinBound && a.CreationDate < yearMinValue {
			continue
		}
		if yearMaxValue < yearMaxBound && a.CreationDate > yearMaxValue {
			continue
		}

		memberCount := len(a.Members)
		if membersMinValue > membersMinBound && memberCount < membersMinValue {
			continue
		}
		if membersMaxValue < membersMaxBound && memberCount > membersMaxValue {
			continue
		}

		filtered = append(filtered, a)
	}

	data := ArtistsPageData{
		Title:           "Artists",
		Source:          "groupie",
		Artists:         filtered,
		Query:           query,
		YearMin:         strconv.Itoa(yearMinValue),
		YearMax:         strconv.Itoa(yearMaxValue),
		MembersMin:      strconv.Itoa(membersMinValue),
		MembersMax:      strconv.Itoa(membersMaxValue),
		Sort:            "",
		ActiveNav:       "artists",
		YearMinBound:    yearMinBound,
		YearMaxBound:    yearMaxBound,
		MembersMinBound: membersMinBound,
		MembersMaxBound: membersMaxBound,
		YearMinValue:    yearMinValue,
		YearMaxValue:    yearMaxValue,
		MembersMinValue: membersMinValue,
		MembersMaxValue: membersMaxValue,
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

	if sortParam == "" {
		sortParam = "relevance"
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
		Title:     "Artists",
		Source:    "spotify",
		Spotify:   views,
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:      sortParam,
		ActiveNav: "artists",
	}

	return data, nil
}

func buildDeezerData(r *http.Request) (ArtistsPageData, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))

	if query == "" {
		query = "a"
	}

	results, err := api.SearchDeezerArtists(query)
	if err != nil {
		return ArtistsPageData{}, err
	}

	views := make([]DeezerArtistView, len(results))
	for i, a := range results {
		views[i].Artist = a
		views[i].Fans = a.NbFan
		views[i].Albums = a.NbAlbum
		views[i].HasRadio = a.Radio
	}

	if sortParam == "" {
		sortParam = "relevance"
	}

	switch sortParam {
	case "fans_asc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Fans < views[j].Fans
		})
	case "fans_desc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Fans > views[j].Fans
		})
	case "albums_asc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Albums < views[j].Albums
		})
	case "albums_desc":
		sort.Slice(views, func(i, j int) bool {
			return views[i].Albums > views[j].Albums
		})
	case "relevance":
	default:
		sortParam = "relevance"
	}

	data := ArtistsPageData{
		Title:     "Artists",
		Source:    "deezer",
		Deezer:    views,
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:      sortParam,
		ActiveNav: "artists",
	}

	return data, nil
}

func buildAppleData(r *http.Request) (ArtistsPageData, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))

	if query == "" {
		query = "a"
	}

	results, err := api.SearchAppleArtistsWithArtwork(query, 30, 300)
	if err != nil {
		return ArtistsPageData{}, err
	}

	views := make([]AppleArtistView, len(results))
	for i, a := range results {
		views[i].Artist = a.Artist
		views[i].Genre = a.Artist.PrimaryGenreName
		views[i].ImageURL = a.ArtworkURL
	}

	if sortParam == "" {
		sortParam = "relevance"
	}

	switch sortParam {
	case "name_asc":
		sort.Slice(views, func(i, j int) bool {
			return strings.ToLower(views[i].Artist.ArtistName) < strings.ToLower(views[j].Artist.ArtistName)
		})
	case "name_desc":
		sort.Slice(views, func(i, j int) bool {
			return strings.ToLower(views[i].Artist.ArtistName) > strings.ToLower(views[j].Artist.ArtistName)
		})
	case "relevance":
	default:
		sortParam = "relevance"
	}

	data := ArtistsPageData{
		Title:     "Artists",
		Source:    "apple",
		Apple:     views,
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:      sortParam,
		ActiveNav: "artists",
	}

	return data, nil
}

func computeGroupieBounds(artists []api.Artist) (int, int, int, int) {
	if len(artists) == 0 {
		return 1900, 2100, 1, 10
	}

	yearMin := artists[0].CreationDate
	yearMax := artists[0].CreationDate

	maxMembers := len(artists[0].Members)
	for i := 1; i < len(artists); i++ {
		a := artists[i]
		if a.CreationDate > 0 && (yearMin == 0 || a.CreationDate < yearMin) {
			yearMin = a.CreationDate
		}
		if a.CreationDate > yearMax {
			yearMax = a.CreationDate
		}
		if m := len(a.Members); m > maxMembers {
			maxMembers = m
		}
	}

	if yearMin <= 0 {
		yearMin = 1900
	}
	if yearMax <= 0 {
		yearMax = 2100
	}
	if yearMax < yearMin {
		yearMax = yearMin
	}

	if maxMembers < 1 {
		maxMembers = 1
	}

	return yearMin, yearMax, 1, maxMembers
}
