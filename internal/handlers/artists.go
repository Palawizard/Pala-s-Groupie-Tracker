package handlers

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"palasgroupietracker/internal/api"
	"palasgroupietracker/internal/geo"
	"palasgroupietracker/internal/store"
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
	BasePath        string
	CurrentURL      string
	User            *store.User
	IsAuthed        bool
	FavoriteIDs     map[string]bool
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

	AlbumMinBound string
	AlbumMaxBound string
	AlbumFrom     string
	AlbumTo       string
	Location      string
}

// ArtistsHandler renders the full artists page using the shared layout
func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)
	user, authed := getCurrentUser(w, r)

	var data ArtistsPageData
	var err error

	if source == "spotify" {
		// Spotify mode uses the Spotify search API and adds Last.fm listeners
		data, err = buildSpotifyData(r)
	} else if source == "deezer" {
		data, err = buildDeezerData(r)
	} else if source == "apple" {
		data, err = buildAppleData(r)
	} else {
		// Default mode uses the original Groupie API and supports year/member filters
		data, err = buildGroupieData(r)
	}

	if err != nil {
		http.Error(w, "failed to load artists", http.StatusInternalServerError)
		return
	}

	data.BasePath = basePath
	data.CurrentURL = buildCurrentURL(r)
	data.User = user
	data.IsAuthed = authed
	data.FavoriteIDs = favoriteIDMap(r, user, source)

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

// ArtistsAjaxHandler renders only the artists list section for live filtering
func ArtistsAjaxHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)
	user, authed := getCurrentUser(w, r)

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

	data.BasePath = basePath
	data.CurrentURL = buildArtistsListURL(r)
	data.User = user
	data.IsAuthed = authed
	data.FavoriteIDs = favoriteIDMap(r, user, source)

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

// buildGroupieData builds the artists list and filter state for the original Groupie dataset
func buildGroupieData(r *http.Request) (ArtistsPageData, error) {
	artists, err := api.FetchArtists()
	if err != nil {
		return ArtistsPageData{}, err
	}

	// Filters are passed as query params so they can be shared via URL
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	yearMinStr := strings.TrimSpace(r.URL.Query().Get("year_min"))
	yearMaxStr := strings.TrimSpace(r.URL.Query().Get("year_max"))
	membersMinStr := strings.TrimSpace(r.URL.Query().Get("members_min"))
	membersMaxStr := strings.TrimSpace(r.URL.Query().Get("members_max"))
	albumFromStr := strings.TrimSpace(r.URL.Query().Get("album_from"))
	albumToStr := strings.TrimSpace(r.URL.Query().Get("album_to"))
	locationQuery := strings.TrimSpace(r.URL.Query().Get("location"))

	yearMinBound, yearMaxBound, membersMinBound, membersMaxBound := computeGroupieBounds(artists)

	// Parse ints and fall back to 0 on invalid values
	yearMin, _ := strconv.Atoi(yearMinStr)
	yearMax, _ := strconv.Atoi(yearMaxStr)
	membersMin, _ := strconv.Atoi(membersMinStr)
	membersMax, _ := strconv.Atoi(membersMaxStr)

	albumMinBoundDate, albumMaxBoundDate := computeFirstAlbumBounds(artists)
	albumMinBound := albumMinBoundDate.Format("2006-01-02")
	albumMaxBound := albumMaxBoundDate.Format("2006-01-02")

	albumFromValue := albumMinBoundDate
	albumToValue := albumMaxBoundDate
	if t, ok := parseISODate(albumFromStr); ok {
		albumFromValue = t
	}
	if t, ok := parseISODate(albumToStr); ok {
		albumToValue = t
	}

	if albumFromValue.Before(albumMinBoundDate) {
		albumFromValue = albumMinBoundDate
	}
	if albumToValue.After(albumMaxBoundDate) {
		albumToValue = albumMaxBoundDate
	}
	if albumFromValue.After(albumToValue) {
		albumFromValue = albumToValue
	}

	yearMinValue := yearMin
	yearMaxValue := yearMax
	membersMinValue := membersMin
	membersMaxValue := membersMax

	if yearMinValue == 0 {
		// Default to the computed min bound when the slider isn't set
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
		// Clamp values so the UI stays in sync with the backend
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
		// Keep ranges consistent even if sliders cross
		yearMinValue = yearMaxValue
	}
	if membersMinValue > membersMaxValue {
		membersMinValue = membersMaxValue
	}

	filtered := make([]api.Artist, 0, len(artists))
	lowerQuery := strings.ToLower(query)

	locationNorm := normalizeForMatch(locationQuery)
	var locationsByArtistID map[int][]string
	if locationNorm != "" {
		relations, relErr := api.FetchRelations()
		if relErr != nil {
			return ArtistsPageData{}, relErr
		}
		locationsByArtistID = make(map[int][]string, len(relations.Index))
		for _, rel := range relations.Index {
			keys := make([]string, 0, len(rel.DatesLocations))
			for k := range rel.DatesLocations {
				keys = append(keys, k)
			}
			locationsByArtistID[rel.ID] = keys
		}
	}

	albumFilterActive := albumFromValue.After(albumMinBoundDate) || albumToValue.Before(albumMaxBoundDate)

	for _, a := range artists {
		if lowerQuery != "" {
			// Match on artist name or member names
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

		if albumFilterActive {
			albumDate, ok := parseFirstAlbumDate(a.FirstAlbum)
			if !ok {
				continue
			}
			if albumFromValue.After(albumMinBoundDate) && albumDate.Before(albumFromValue) {
				continue
			}
			if albumToValue.Before(albumMaxBoundDate) && albumDate.After(albumToValue) {
				continue
			}
		}

		if locationNorm != "" {
			keys := locationsByArtistID[a.ID]
			if len(keys) == 0 {
				continue
			}
			matched := false
			for _, key := range keys {
				if strings.Contains(normalizeForMatch(key), locationNorm) {
					matched = true
					break
				}
				_, _, display := geo.QueryFromLocationKey(key)
				if display != "" && strings.Contains(normalizeForMatch(display), locationNorm) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
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

		AlbumMinBound: albumMinBound,
		AlbumMaxBound: albumMaxBound,
		AlbumFrom:     albumFromValue.Format("2006-01-02"),
		AlbumTo:       albumToValue.Format("2006-01-02"),
		Location:      locationQuery,
	}

	return data, nil
}

// buildSpotifyData searches Spotify and enriches results with Last.fm listener counts
func buildSpotifyData(r *http.Request) (ArtistsPageData, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))

	if query == "" {
		// Spotify search rejects empty queries
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

	// Fetch Last.fm listeners in parallel but cap concurrency
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i := range views {
		wg.Add(1)
		go func(v *SpotifyArtistView) { // fetch Last.fm listeners concurrently
			defer wg.Done()
			sem <- struct{}{}
			listeners, err := api.FetchArtistMonthlyListeners(v.Artist.Name)
			if err != nil {
				// Listener counts are best-effort, keep the artist even on failure
				listeners = 0
			}
			v.MonthlyListeners = listeners
			<-sem
		}(&views[i])
	}

	wg.Wait()

	if sortParam == "" {
		// Default matches Spotify's natural "relevance" ordering
		sortParam = "relevance"
	}

	switch sortParam {
	case "followers_asc":
		sort.Slice(views, func(i, j int) bool { // smallest follower count first
			return views[i].Followers < views[j].Followers
		})
	case "followers_desc":
		sort.Slice(views, func(i, j int) bool { // largest follower count first
			return views[i].Followers > views[j].Followers
		})
	case "listeners_asc":
		sort.Slice(views, func(i, j int) bool { // smallest listener count first
			return views[i].MonthlyListeners < views[j].MonthlyListeners
		})
	case "listeners_desc":
		sort.Slice(views, func(i, j int) bool { // largest listener count first
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
		// Preserve the user's original query instead of the fallback "a"
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:      sortParam,
		ActiveNav: "artists",
	}

	return data, nil
}

// buildDeezerData searches Deezer and applies simple sorting options
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
		sort.Slice(views, func(i, j int) bool { // smallest fan count first
			return views[i].Fans < views[j].Fans
		})
	case "fans_desc":
		sort.Slice(views, func(i, j int) bool { // largest fan count first
			return views[i].Fans > views[j].Fans
		})
	case "albums_asc":
		sort.Slice(views, func(i, j int) bool { // smallest album count first
			return views[i].Albums < views[j].Albums
		})
	case "albums_desc":
		sort.Slice(views, func(i, j int) bool { // largest album count first
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

// buildAppleData searches iTunes for artists and uses artwork as a proxy for artist images
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
		sort.Slice(views, func(i, j int) bool { // A to Z by artist name
			return strings.ToLower(views[i].Artist.ArtistName) < strings.ToLower(views[j].Artist.ArtistName)
		})
	case "name_desc":
		sort.Slice(views, func(i, j int) bool { // Z to A by artist name
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

// computeGroupieBounds computes slider bounds for year and member count
func computeGroupieBounds(artists []api.Artist) (int, int, int, int) {
	if len(artists) == 0 {
		// Keep sane defaults so the UI can still render
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
		// Avoid broken ranges if the dataset is malformed
		yearMax = yearMin
	}

	if maxMembers < 1 {
		maxMembers = 1
	}

	return yearMin, yearMax, 1, maxMembers
}

func parseISODate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func parseFirstAlbumDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// Groupie usually uses DD-MM-YYYY.
	if t, err := time.Parse("02-01-2006", s); err == nil {
		return t, true
	}
	// Accept ISO as a fallback if the dataset changes.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func computeFirstAlbumBounds(artists []api.Artist) (time.Time, time.Time) {
	var minDate time.Time
	var maxDate time.Time

	for _, a := range artists {
		d, ok := parseFirstAlbumDate(a.FirstAlbum)
		if !ok {
			continue
		}
		if minDate.IsZero() || d.Before(minDate) {
			minDate = d
		}
		if maxDate.IsZero() || d.After(maxDate) {
			maxDate = d
		}
	}

	if minDate.IsZero() || maxDate.IsZero() {
		// Keep sane defaults if parsing fails.
		minDate = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		maxDate = time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	if maxDate.Before(minDate) {
		maxDate = minDate
	}

	return minDate, maxDate
}

func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	r := strings.NewReplacer(
		"_", " ",
		"-", " ",
		",", " ",
		".", " ",
		"/", " ",
		"\\", " ",
	)
	s = r.Replace(s)
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}
