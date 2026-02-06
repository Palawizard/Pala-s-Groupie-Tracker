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

// Shared geocoder instance keeps a warm cache across requests
var groupieGeocoder = geo.NewGeocoder()

type ArtistDetailPageData struct {
	Title     string
	Source    string
	ActiveNav string
	BasePath  string
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

// ArtistDetailHandler routes to the correct detail handler based on the `source` query parameter
func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	// The router is registered as `/artists/`, so the last segment is the ID
	idSegment := path.Base(r.URL.Path)

	if idSegment == "" || idSegment == "artists" {
		// `/artists/` without an ID should go back to the list page
		http.Redirect(w, r, withBasePath(r, "/artists")+"?source="+source, http.StatusSeeOther)
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

// handleGroupieArtistDetail renders the detail page for artists from the Groupie Tracker dataset
func handleGroupieArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		// IDs are numeric in Groupie mode
		http.Redirect(w, r, withBasePath(r, "/artists")+"?source=groupie", http.StatusSeeOther)
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
	// Sort keys for stable output and predictable map ordering
	keys := make([]string, 0, len(relation.DatesLocations))
	for k := range relation.DatesLocations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	const maxLocations = 25
	if len(keys) > maxLocations {
		// Avoid geocoding too many points in one request
		keys = keys[:maxLocations]
	}

	locations = make([]MapLocation, 0, len(keys))
	var mu sync.Mutex
	// Cap concurrency since geocoding calls external providers
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, name := range keys {
		dates := relation.DatesLocations[name]
		// Convert Groupie location keys into a geocoding-friendly query
		place, countryCode, display := geo.QueryFromLocationKey(name)

		wg.Add(1)
		go func(place, countryCode, display string, dates []string) { // geocode locations concurrently
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }() // release concurrency slot

			res, ok, geoErr := groupieGeocoder.Geocode(r.Context(), place, countryCode)
			if geoErr != nil || !ok {
				// Missing geocodes are expected for noisy location strings
				return
			}

			if strings.TrimSpace(res.Display) == "" {
				// Fall back to our own label if the provider didn't return one
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

	// Sort final locations alphabetically for consistent popups
	sort.SliceStable(locations, func(i, j int) bool { // case-insensitive by display name
		return strings.ToLower(locations[i].Name) < strings.ToLower(locations[j].Name)
	})

	locBytes, err := json.Marshal(locations)
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	// Wikipedia is best-effort, the page should still render without it
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
		BasePath:  getBasePath(r),
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

		// LocationsJSON is embedded into a script tag for the Leaflet map
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

// handleSpotifyArtistDetail renders the detail page for a Spotify artist ID
func handleSpotifyArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	if !isLikelySpotifyID(idSegment) {
		// Protect the API from random strings and keep URLs predictable
		http.Redirect(w, r, withBasePath(r, "/artists")+"?source=spotify", http.StatusSeeOther)
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

	// Non-Groupie sources don't have concert locations
	emptyLocations, err := json.Marshal([]MapLocation{})
	if err != nil {
		http.Error(w, "failed to encode concerts", http.StatusInternalServerError)
		return
	}

	wikiSummary, wikiURL, wikiErr := api.FetchWikipediaSummary(artist.Name)
	hasWiki := wikiErr == nil && wikiSummary != "" && wikiURL != ""

	genre := ""
	if len(artist.Genres) > 0 {
		// Capitalize the first genre for nicer display
		runes := []rune(artist.Genres[0])
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		genre = string(runes)
	}

	listeners, err := api.FetchArtistMonthlyListeners(artist.Name)
	if err != nil {
		// Last.fm can fail independently, keep the rest of the page
		listeners = 0
	}

	followers := 0
	if artist.Followers != nil {
		followers = artist.Followers.Total
	}

	topTracks, err := api.GetSpotifyArtistTopTracks(artist.ID, "FR")
	if err != nil {
		// Tracks are optional for the page to work
		topTracks = nil
	}

	latestAlbums, err := api.GetSpotifyArtistAlbums(artist.ID, "FR", 8)
	if err != nil {
		latestAlbums = nil
	}

	if len(latestAlbums) > 1 {
		// Keep album ordering deterministic even if the API ordering changes
		sort.SliceStable(latestAlbums, func(i, j int) bool { // newest first, then stable tie-breakers
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
		BasePath:  getBasePath(r),
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

// handleDeezerArtistDetail renders the detail page for a Deezer artist ID
func handleDeezerArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.Redirect(w, r, withBasePath(r, "/artists")+"?source=deezer", http.StatusSeeOther)
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
		// Track lists are optional for the rest of the page
		topTracks = nil
	}

	latestAlbums, err := api.GetDeezerArtistAlbums(artist.ID, 8)
	if err != nil {
		latestAlbums = nil
	}

	if len(latestAlbums) > 1 {
		// Sort on the backend so template rendering stays simple
		sort.SliceStable(latestAlbums, func(i, j int) bool { // newest first, then stable tie-breakers
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
		BasePath:  getBasePath(r),
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

// handleAppleArtistDetail renders the detail page for an iTunes artist ID
func handleAppleArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.Redirect(w, r, withBasePath(r, "/artists")+"?source=apple", http.StatusSeeOther)
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
		// Use the newest album cover as the hero image when possible
		hero = upscaleAppleArtwork(latestAlbums[0].ArtworkURL100, 600)
	}
	if hero == "" && len(topTracks) > 0 {
		// Fall back to a track artwork if we didn't get an album cover
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
		BasePath:  getBasePath(r),
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

// upscaleAppleArtwork rewrites `100x100bb.jpg` style artwork URLs to a larger size
func upscaleAppleArtwork(u string, size int) string {
	u = strings.TrimSpace(u)
	if u == "" || size <= 0 {
		return ""
	}

	// iTunes encodes the requested size in the last path segment
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

// isLikelySpotifyID does a fast sanity check for the 22-char base62 Spotify IDs
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

// isNotFoundError checks common "not found" shapes across the external APIs used by the project
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
