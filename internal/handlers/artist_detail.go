package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"palasgroupietracker/internal/api"
	"palasgroupietracker/internal/geo"
)

type MapLocation struct {
	Name  string   `json:"name"`
	Lat   float64  `json:"lat"`
	Lng   float64  `json:"lng"`
	Dates []string `json:"dates"`
}

var groupieGeocoder = geo.NewGeocoder()

type ArtistDetailPageData struct {
	Title     string
	Source    string
	ActiveNav string
	Artist    *api.Artist

	SpotifyArtist           *api.SpotifyArtist
	SpotifyGenre            string
	SpotifyFollowers        int
	SpotifyMonthlyListeners int
	SpotifyTopTracks        []api.SpotifyTrack
	SpotifyLatestAlbums     []api.SpotifyAlbum

	DeezerArtist           *api.DeezerArtist
	DeezerFans             int
	DeezerAlbumsCount      int
	DeezerHasRadio         bool
	DeezerMonthlyListeners int
	DeezerTopTracks        []api.DeezerTrack
	DeezerLatestAlbums     []api.DeezerAlbum

	AppleArtist           *api.AppleArtist
	AppleGenre            string
	AppleMonthlyListeners int
	AppleHeroImage        string
	AppleTopTracks        []api.AppleTrack
	AppleLatestAlbums     []api.AppleAlbum

	LocationsJSON template.JS
	WikiSummary   string
	WikiURL       string
	HasWiki       bool
}

func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	idSegment := path.Base(r.URL.Path)

	// "/artists/" can reach this handler due to the mux prefix match. In that case
	// (or if the segment clearly doesn't match the selected source), redirect to the
	// artists search page instead of returning a 404.
	if idSegment == "" || idSegment == "artists" {
		http.Redirect(w, r, "/artists?source="+source, http.StatusSeeOther)
		return
	}

	if source == "spotify" {
		handleSpotifyArtistDetail(w, r, idSegment)
		return
	}
	if source == "deezer" {
		handleDeezerArtistDetail(w, r, idSegment)
		return
	}
	if source == "apple" {
		handleAppleArtistDetail(w, r, idSegment)
		return
	}

	handleGroupieArtistDetail(w, r, idSegment)
}

func handleGroupieArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.Redirect(w, r, "/artists?source=groupie", http.StatusSeeOther)
		return
	}

	artist, err := api.FetchArtistByID(id)
	if err != nil {
		NotFound(w, r)
		return
	}

	relation, err := api.FetchRelationForArtist(id)
	if err != nil {
		http.Error(w, "failed to load concerts", http.StatusInternalServerError)
		return
	}

	var locations []MapLocation
	keys := make([]string, 0, len(relation.DatesLocations))
	for k := range relation.DatesLocations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Bound geocoding work per request so the page stays responsive even if an artist
	// has a lot of locations.
	const maxLocations = 25
	if len(keys) > maxLocations {
		keys = keys[:maxLocations]
	}

	locations = make([]MapLocation, 0, len(keys))
	var mu sync.Mutex
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, name := range keys {
		dates := relation.DatesLocations[name]
		place, countryCode, display := geo.QueryFromLocationKey(name)

		wg.Add(1)
		go func(place, countryCode, display string, dates []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, ok, geoErr := groupieGeocoder.Geocode(r.Context(), place, countryCode)
			if geoErr != nil || !ok {
				return
			}

			if strings.TrimSpace(res.Display) == "" {
				res.Display = display
			}

			mu.Lock()
			locations = append(locations, MapLocation{
				Name:  res.Display,
				Lat:   res.Lat,
				Lng:   res.Lng,
				Dates: dates,
			})
			mu.Unlock()
		}(place, countryCode, display, dates)
	}

	wg.Wait()

	sort.SliceStable(locations, func(i, j int) bool {
		return strings.ToLower(locations[i].Name) < strings.ToLower(locations[j].Name)
	})

	locBytes, err := json.Marshal(locations)
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	wikiSummary, wikiURL, wikiErr := api.FetchWikipediaSummary(artist.Name)
	hasWiki := wikiErr == nil && wikiSummary != "" && wikiURL != ""

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:     artist.Name,
		Source:    "groupie",
		ActiveNav: "artists",
		Artist:    artist,

		SpotifyArtist:           nil,
		SpotifyGenre:            "",
		SpotifyFollowers:        0,
		SpotifyMonthlyListeners: 0,
		SpotifyTopTracks:        nil,
		SpotifyLatestAlbums:     nil,

		DeezerArtist:           nil,
		DeezerFans:             0,
		DeezerAlbumsCount:      0,
		DeezerHasRadio:         false,
		DeezerMonthlyListeners: 0,
		DeezerTopTracks:        nil,
		DeezerLatestAlbums:     nil,

		AppleArtist:           nil,
		AppleGenre:            "",
		AppleMonthlyListeners: 0,
		AppleHeroImage:        "",
		AppleTopTracks:        nil,
		AppleLatestAlbums:     nil,

		LocationsJSON: template.JS(locBytes),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func handleSpotifyArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	if !isLikelySpotifyID(idSegment) {
		http.Redirect(w, r, "/artists?source=spotify", http.StatusSeeOther)
		return
	}

	artist, err := api.GetSpotifyArtist(idSegment)
	if err != nil {
		if isNotFoundError(err) {
			NotFound(w, r)
			return
		}
		http.Error(w, "failed to load spotify artist", http.StatusInternalServerError)
		return
	}

	emptyLocations, err := json.Marshal([]MapLocation{})
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	wikiSummary, wikiURL, wikiErr := api.FetchWikipediaSummary(artist.Name)
	hasWiki := wikiErr == nil && wikiSummary != "" && wikiURL != ""

	genre := ""
	if len(artist.Genres) > 0 {
		runes := []rune(artist.Genres[0])
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		genre = string(runes)
	}

	listeners, err := api.FetchArtistMonthlyListeners(artist.Name)
	if err != nil {
		listeners = 0
	}

	followers := 0
	if artist.Followers != nil {
		followers = artist.Followers.Total
	}

	topTracks, err := api.GetSpotifyArtistTopTracks(artist.ID, "FR")
	if err != nil {
		topTracks = nil
	}

	latestAlbums, err := api.GetSpotifyArtistAlbums(artist.ID, "FR", 8)
	if err != nil {
		latestAlbums = nil
	}

	if len(latestAlbums) > 1 {
		sort.SliceStable(latestAlbums, func(i, j int) bool {
			di, okI := api.ParseSpotifyReleaseDate(latestAlbums[i].ReleaseDate)
			dj, okJ := api.ParseSpotifyReleaseDate(latestAlbums[j].ReleaseDate)

			if okI && okJ && !di.Equal(dj) {
				return di.After(dj)
			}
			if okI != okJ {
				return okI
			}

			ni := strings.ToLower(latestAlbums[i].Name)
			nj := strings.ToLower(latestAlbums[j].Name)
			if ni != nj {
				return ni < nj
			}

			return latestAlbums[i].ID < latestAlbums[j].ID
		})
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:     artist.Name,
		Source:    "spotify",
		ActiveNav: "artists",
		Artist:    nil,

		SpotifyArtist:           artist,
		SpotifyGenre:            genre,
		SpotifyFollowers:        followers,
		SpotifyMonthlyListeners: listeners,
		SpotifyTopTracks:        topTracks,
		SpotifyLatestAlbums:     latestAlbums,

		DeezerArtist:           nil,
		DeezerFans:             0,
		DeezerAlbumsCount:      0,
		DeezerHasRadio:         false,
		DeezerMonthlyListeners: 0,
		DeezerTopTracks:        nil,
		DeezerLatestAlbums:     nil,

		AppleArtist:           nil,
		AppleGenre:            "",
		AppleMonthlyListeners: 0,
		AppleHeroImage:        "",
		AppleTopTracks:        nil,
		AppleLatestAlbums:     nil,

		LocationsJSON: template.JS(emptyLocations),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func handleDeezerArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.Redirect(w, r, "/artists?source=deezer", http.StatusSeeOther)
		return
	}

	artist, err := api.GetDeezerArtist(id)
	if err != nil {
		if isNotFoundError(err) {
			NotFound(w, r)
			return
		}
		http.Error(w, "failed to load deezer artist", http.StatusInternalServerError)
		return
	}

	emptyLocations, err := json.Marshal([]MapLocation{})
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	wikiSummary, wikiURL, wikiErr := api.FetchWikipediaSummary(artist.Name)
	hasWiki := wikiErr == nil && wikiSummary != "" && wikiURL != ""

	monthly, err := api.FetchArtistMonthlyListeners(artist.Name)
	if err != nil {
		monthly = 0
	}

	topTracks, err := api.GetDeezerArtistTopTracks(artist.ID, 10)
	if err != nil {
		topTracks = nil
	}

	latestAlbums, err := api.GetDeezerArtistAlbums(artist.ID, 8)
	if err != nil {
		latestAlbums = nil
	}

	if len(latestAlbums) > 1 {
		sort.SliceStable(latestAlbums, func(i, j int) bool {
			di, okI := api.ParseDeezerReleaseDate(latestAlbums[i].ReleaseDate)
			dj, okJ := api.ParseDeezerReleaseDate(latestAlbums[j].ReleaseDate)

			if okI && okJ && !di.Equal(dj) {
				return di.After(dj)
			}
			if okI != okJ {
				return okI
			}

			ni := strings.ToLower(latestAlbums[i].Title)
			nj := strings.ToLower(latestAlbums[j].Title)
			if ni != nj {
				return ni < nj
			}

			return latestAlbums[i].ID < latestAlbums[j].ID
		})
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:     artist.Name,
		Source:    "deezer",
		ActiveNav: "artists",
		Artist:    nil,

		SpotifyArtist:           nil,
		SpotifyGenre:            "",
		SpotifyFollowers:        0,
		SpotifyMonthlyListeners: 0,
		SpotifyTopTracks:        nil,
		SpotifyLatestAlbums:     nil,

		DeezerArtist:           artist,
		DeezerFans:             artist.NbFan,
		DeezerAlbumsCount:      artist.NbAlbum,
		DeezerHasRadio:         artist.Radio,
		DeezerMonthlyListeners: monthly,
		DeezerTopTracks:        topTracks,
		DeezerLatestAlbums:     latestAlbums,

		AppleArtist:           nil,
		AppleGenre:            "",
		AppleMonthlyListeners: 0,
		AppleHeroImage:        "",
		AppleTopTracks:        nil,
		AppleLatestAlbums:     nil,

		LocationsJSON: template.JS(emptyLocations),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func handleAppleArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.Redirect(w, r, "/artists?source=apple", http.StatusSeeOther)
		return
	}

	artist, err := api.GetAppleArtist(id)
	if err != nil {
		if isNotFoundError(err) {
			NotFound(w, r)
			return
		}
		http.Error(w, "failed to load apple artist", http.StatusInternalServerError)
		return
	}

	emptyLocations, err := json.Marshal([]MapLocation{})
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	wikiSummary, wikiURL, wikiErr := api.FetchWikipediaSummary(artist.ArtistName)
	hasWiki := wikiErr == nil && wikiSummary != "" && wikiURL != ""

	monthly, err := api.FetchArtistMonthlyListeners(artist.ArtistName)
	if err != nil {
		monthly = 0
	}

	latestAlbums, err := api.GetAppleArtistAlbums(artist.ArtistID, 8)
	if err != nil {
		latestAlbums = nil
	}

	topTracks, err := api.GetAppleArtistSongs(artist.ArtistID, 10)
	if err != nil {
		topTracks = nil
	}

	hero := ""
	if len(latestAlbums) > 0 {
		hero = upscaleAppleArtwork(latestAlbums[0].ArtworkURL100, 600)
	}
	if hero == "" && len(topTracks) > 0 {
		hero = upscaleAppleArtwork(topTracks[0].ArtworkURL100, 600)
	}

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:     artist.ArtistName,
		Source:    "apple",
		ActiveNav: "artists",
		Artist:    nil,

		SpotifyArtist:           nil,
		SpotifyGenre:            "",
		SpotifyFollowers:        0,
		SpotifyMonthlyListeners: 0,
		SpotifyTopTracks:        nil,
		SpotifyLatestAlbums:     nil,

		DeezerArtist:           nil,
		DeezerFans:             0,
		DeezerAlbumsCount:      0,
		DeezerHasRadio:         false,
		DeezerMonthlyListeners: 0,
		DeezerTopTracks:        nil,
		DeezerLatestAlbums:     nil,

		AppleArtist:           artist,
		AppleGenre:            artist.PrimaryGenreName,
		AppleMonthlyListeners: monthly,
		AppleHeroImage:        hero,
		AppleTopTracks:        topTracks,
		AppleLatestAlbums:     latestAlbums,

		LocationsJSON: template.JS(emptyLocations),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func upscaleAppleArtwork(u string, size int) string {
	u = strings.TrimSpace(u)
	if u == "" || size <= 0 {
		return ""
	}

	target := strconv.Itoa(size) + "x" + strconv.Itoa(size) + "bb.jpg"
	parts := strings.Split(u, "/")
	if len(parts) == 0 {
		return u
	}
	last := parts[len(parts)-1]
	if strings.Contains(last, "x") && strings.HasSuffix(last, "bb.jpg") {
		parts[len(parts)-1] = target
		return strings.Join(parts, "/")
	}
	return u
}

func isLikelySpotifyID(s string) bool {
	if len(s) != 22 {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func isNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "404") {
		return true
	}
	if strings.Contains(msg, "not found") {
		return true
	}
	if strings.Contains(msg, "unknown id") {
		return true
	}
	if strings.Contains(msg, "invalid id") {
		return true
	}
	return false
}
