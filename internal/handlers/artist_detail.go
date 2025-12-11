package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
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
	Title         string
	Source        string
	Artist        *api.Artist
	SpotifyArtist *api.SpotifyArtist
	SpotifyGenre  string
	LocationsJSON template.JS
	WikiSummary   string
	WikiURL       string
	HasWiki       bool
}

func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	idSegment := path.Base(r.URL.Path)
	sourceParam := getSource(r)

	if _, err := strconv.Atoi(idSegment); err != nil || sourceParam == "spotify" {
		handleSpotifyArtistDetail(w, r, idSegment)
		return
	}

	handleGroupieArtistDetail(w, r, idSegment)
}

func handleGroupieArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	id, err := strconv.Atoi(idSegment)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	artist, err := api.FetchArtistByID(id)
	if err != nil {
		http.NotFound(w, r)
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
		Title:         artist.Name,
		Source:        "groupie",
		Artist:        artist,
		SpotifyArtist: nil,
		SpotifyGenre:  "",
		LocationsJSON: template.JS(locBytes),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func handleSpotifyArtistDetail(w http.ResponseWriter, r *http.Request, idSegment string) {
	artist, err := api.GetSpotifyArtist(idSegment)
	if err != nil {
		log.Printf("artist detail: failed to load spotify artist id=%s: %v\n", idSegment, err)
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

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/artist_detail.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := ArtistDetailPageData{
		Title:         artist.Name,
		Source:        "spotify",
		Artist:        nil,
		SpotifyArtist: artist,
		SpotifyGenre:  genre,
		LocationsJSON: template.JS(emptyLocations),
		WikiSummary:   wikiSummary,
		WikiURL:       wikiURL,
		HasWiki:       hasWiki,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
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
