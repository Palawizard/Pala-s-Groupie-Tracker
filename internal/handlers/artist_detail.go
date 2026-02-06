package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"palasgroupietracker/internal/api"
)

type MapLocation struct {
	Name  string   `json:"name"`
	Lat   float64  `json:"lat"`
	Lng   float64  `json:"lng"`
	Dates []string `json:"dates"`
}

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
		NotFound(w, r)
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
	for name, dates := range relation.DatesLocations {
		lat, lng, ok := lookupCoords(name)
		if !ok {
			continue
		}
		locations = append(locations, MapLocation{
			Name:  name,
			Lat:   lat,
			Lng:   lng,
			Dates: dates,
		})
	}

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
		NotFound(w, r)
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
		NotFound(w, r)
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
			di, okI := parseDeezerReleaseDate(latestAlbums[i].ReleaseDate)
			dj, okJ := parseDeezerReleaseDate(latestAlbums[j].ReleaseDate)

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
		NotFound(w, r)
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

func parseDeezerReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000-00-00" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
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

func lookupCoords(location string) (float64, float64, bool) {
	coords := map[string][2]float64{
		"london-uk":                 {51.5074, -0.1278},
		"lausanne-switzerland":      {46.5197, 6.6323},
		"lyon-france":               {45.764, 4.8357},
		"los_angeles-usa":           {34.0522, -118.2437},
		"georgia-usa":               {32.1656, -82.9001},
		"north_carolina-usa":        {35.7596, -79.0193},
		"victoria-australia":        {-37.8136, 144.9631},
		"queensland-australia":      {-20.9176, 142.7028},
		"new_south_wales-australia": {-31.2532, 146.9211},
		"auckland-new_zealand":      {-36.8485, 174.7633},
		"dunedin-new_zealand":       {-45.8788, 170.5028},
		"penrose-new_zealand":       {-36.9075, 174.8167},
		"saitama-japan":             {35.8617, 139.6455},
		"osaka-japan":               {34.6937, 135.5023},
		"nagoya-japan":              {35.1815, 136.9066},
		"yogyakarta-indonesia":      {-7.7956, 110.3695},
		"budapest-hungary":          {47.4979, 19.0402},
		"minsk-belarus":             {53.9006, 27.559},
		"bratislava-slovakia":       {48.1486, 17.1077},
		"noumea-new_caledonia":      {-22.2711, 166.438},
		"papeete-french_polynesia":  {-17.5516, -149.5585},
		"playa_del_carmen-mexico":   {20.6296, -87.0739},
	}

	if c, ok := coords[location]; ok {
		return c[0], c[1], true
	}

	return 0, 0, false
}
