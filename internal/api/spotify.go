package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type SpotifyFollowers struct {
	Total int `json:"total"`
}

type SpotifyArtist struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Genres       []string          `json:"genres"`
	Images       []SpotifyImage    `json:"images"`
	Followers    *SpotifyFollowers `json:"followers"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

type SpotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type SpotifyAlbum struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	AlbumType    string         `json:"album_type"`
	ReleaseDate  string         `json:"release_date"`
	TotalTracks  int            `json:"total_tracks"`
	Images       []SpotifyImage `json:"images"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

type SpotifyTrack struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DurationMS   int    `json:"duration_ms"`
	PreviewURL   string `json:"preview_url"`
	Popularity   int    `json:"popularity"`
	TrackNumber  int    `json:"track_number"`
	DiscNumber   int    `json:"disc_number"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
	Album SpotifyAlbum `json:"album"`
}

type spotifySearchResponse struct {
	Artists struct {
		Items []SpotifyArtist `json:"items"`
	} `json:"artists"`
}

type spotifyTopTracksResponse struct {
	Tracks []SpotifyTrack `json:"tracks"`
}

type spotifyArtistAlbumsResponse struct {
	Items []SpotifyAlbum `json:"items"`
}

var spotifyHTTP = &http.Client{Timeout: 8 * time.Second}

var spotifyTokenCache = struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}{}

// spotifyClose closes a response body and ignores any close error
func spotifyClose(c io.Closer) {
	_ = c.Close()
}

// spotifyNewJSONRequest creates an HTTP request with Spotify-friendly headers
func spotifyNewJSONRequest(method, u string, body io.Reader, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		// Most Spotify endpoints require a bearer token
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// spotifyDoJSON executes req, checks the expected status code, then decodes JSON into out
func spotifyDoJSON(req *http.Request, expectedStatus int, out any) error {
	resp, err := spotifyHTTP.Do(req)
	if err != nil {
		return err
	}
	defer spotifyClose(resp.Body)

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("spotify request failed: %s", resp.Status)
	}

	if out == nil {
		// Some calls only need the status check
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// getSpotifyToken obtains and caches an app access token using the client credentials flow
func getSpotifyToken() (string, error) {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing spotify credentials")
	}

	spotifyTokenCache.mu.Lock()
	if spotifyTokenCache.token != "" && time.Now().Before(spotifyTokenCache.expiresAt.Add(-30*time.Second)) {
		// Refresh a bit early to avoid edge cases during concurrent requests
		t := spotifyTokenCache.token
		spotifyTokenCache.mu.Unlock()
		return t, nil
	}
	spotifyTokenCache.mu.Unlock()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := spotifyDoJSON(req, http.StatusOK, &body); err != nil {
		return "", err
	}

	if body.AccessToken == "" {
		return "", fmt.Errorf("empty spotify access token")
	}

	// Spotify's expires_in is in seconds
	expiresAt := time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)

	spotifyTokenCache.mu.Lock()
	spotifyTokenCache.token = body.AccessToken
	spotifyTokenCache.expiresAt = expiresAt
	spotifyTokenCache.mu.Unlock()

	return body.AccessToken, nil
}

// SearchSpotifyArtists searches Spotify for artists matching query
func SearchSpotifyArtists(query string) ([]SpotifyArtist, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	baseURL := "https://api.spotify.com/v1/search"
	params := url.Values{}
	q := strings.TrimSpace(query)
	if q == "" {
		// Empty queries return 400, keep the UI functional with a default search
		q = "a"
	}
	params.Set("q", q)
	params.Set("type", "artist")
	params.Set("limit", "30")
	// Market can affect which artists are returned
	params.Set("market", "US")

	req, err := spotifyNewJSONRequest("GET", baseURL+"?"+params.Encode(), nil, token)
	if err != nil {
		return nil, err
	}

	var body spotifySearchResponse
	if err := spotifyDoJSON(req, http.StatusOK, &body); err != nil {
		return nil, err
	}

	return body.Artists.Items, nil
}

// GetSpotifyArtist fetches an artist by Spotify ID
func GetSpotifyArtist(id string) (*SpotifyArtist, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	artistURL := "https://api.spotify.com/v1/artists/" + id
	req, err := spotifyNewJSONRequest("GET", artistURL, nil, token)
	if err != nil {
		return nil, err
	}

	var artist SpotifyArtist
	if err := spotifyDoJSON(req, http.StatusOK, &artist); err != nil {
		return nil, err
	}

	return &artist, nil
}

// GetSpotifyArtistTopTracks returns the artist's top tracks for a given market
func GetSpotifyArtistTopTracks(id string, market string) ([]SpotifyTrack, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	m := strings.TrimSpace(market)
	if m == "" {
		// Spotify requires a market, default to US
		m = "US"
	}

	baseURL := "https://api.spotify.com/v1/artists/" + id + "/top-tracks"
	params := url.Values{}
	params.Set("market", m)

	req, err := spotifyNewJSONRequest("GET", baseURL+"?"+params.Encode(), nil, token)
	if err != nil {
		return nil, err
	}

	var body spotifyTopTracksResponse
	if err := spotifyDoJSON(req, http.StatusOK, &body); err != nil {
		return nil, err
	}

	return body.Tracks, nil
}

// GetSpotifyArtistAlbums returns a de-duplicated and sorted list of an artist's latest albums and singles
func GetSpotifyArtistAlbums(id string, market string, limit int) ([]SpotifyAlbum, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	m := strings.TrimSpace(market)
	if m == "" {
		m = "US"
	}

	want := limit
	if want <= 0 {
		want = 10
	}
	if want > 50 {
		want = 50
	}
	// Spotify album list API caps limit at 50; fetch a full page to reduce duplicates
	// before de-duplication and truncation to `want`
	fetch := 50

	baseURL := "https://api.spotify.com/v1/artists/" + id + "/albums"
	params := url.Values{}
	params.Set("include_groups", "album,single")
	params.Set("market", m)
	params.Set("limit", fmt.Sprintf("%d", fetch))
	params.Set("offset", "0")

	req, err := spotifyNewJSONRequest("GET", baseURL+"?"+params.Encode(), nil, token)
	if err != nil {
		return nil, err
	}

	var body spotifyArtistAlbumsResponse
	if err := spotifyDoJSON(req, http.StatusOK, &body); err != nil {
		return nil, err
	}

	// The API can return duplicates across include_groups, merge by ID
	byID := make(map[string]SpotifyAlbum, len(body.Items))
	for _, a := range body.Items {
		if a.ID == "" {
			continue
		}
		if existing, ok := byID[a.ID]; ok {
			// Keep the entry with the best (latest) release_date we can parse
			da, oka := ParseSpotifyReleaseDate(a.ReleaseDate)
			de, oke := ParseSpotifyReleaseDate(existing.ReleaseDate)
			if oka && (!oke || da.After(de)) {
				byID[a.ID] = a
			}
			continue
		}
		byID[a.ID] = a
	}

	merged := make([]SpotifyAlbum, 0, len(byID))
	for _, a := range byID {
		merged = append(merged, a)
	}

	sort.SliceStable(merged, func(i, j int) bool { // newest first, then stable tie-breakers
		di, okI := ParseSpotifyReleaseDate(merged[i].ReleaseDate)
		dj, okJ := ParseSpotifyReleaseDate(merged[j].ReleaseDate)

		if okI && okJ && !di.Equal(dj) {
			// Prefer newest releases when both dates are parseable
			return di.After(dj)
		}
		if okI != okJ {
			// Prefer albums with parseable dates
			return okI
		}

		ni := strings.ToLower(merged[i].Name)
		nj := strings.ToLower(merged[j].Name)
		if ni != nj {
			return ni < nj
		}

		return merged[i].ID < merged[j].ID
	})

	if len(merged) > want {
		merged = merged[:want]
	}

	return merged, nil
}

// ParseSpotifyReleaseDate parses Spotify's release_date which can be yyyy, yyyy-mm, or yyyy-mm-dd
func ParseSpotifyReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}

	// Some endpoints return timestamps, normalize them to a day-level value
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if len(s) >= 10 {
		// Handle strings like `yyyy-mm-ddTHH:mm:ssZ` by trimming first
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}

	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse("2006-01", s); err == nil {
		// Month-only dates default to the first of the month for sorting
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse("2006", s); err == nil {
		// Year-only dates default to Jan 1 for sorting
		return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC), true
	}

	return time.Time{}, false
}
