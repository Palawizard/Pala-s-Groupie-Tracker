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

func spotifyClose(c io.Closer) {
	_ = c.Close()
}

func spotifyNewJSONRequest(method, u string, body io.Reader, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

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
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func getSpotifyToken() (string, error) {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing spotify credentials")
	}

	spotifyTokenCache.mu.Lock()
	if spotifyTokenCache.token != "" && time.Now().Before(spotifyTokenCache.expiresAt.Add(-30*time.Second)) {
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

	expiresAt := time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)

	spotifyTokenCache.mu.Lock()
	spotifyTokenCache.token = body.AccessToken
	spotifyTokenCache.expiresAt = expiresAt
	spotifyTokenCache.mu.Unlock()

	return body.AccessToken, nil
}

func SearchSpotifyArtists(query string) ([]SpotifyArtist, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	baseURL := "https://api.spotify.com/v1/search"
	params := url.Values{}
	q := strings.TrimSpace(query)
	if q == "" {
		q = "a"
	}
	params.Set("q", q)
	params.Set("type", "artist")
	params.Set("limit", "30")
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

func GetSpotifyArtistTopTracks(id string, market string) ([]SpotifyTrack, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	m := strings.TrimSpace(market)
	if m == "" {
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

func GetSpotifyArtistAlbums(id string, market string, limit int) ([]SpotifyAlbum, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	m := strings.TrimSpace(market)
	if m == "" {
		m = "US"
	}
	// `limit` is the number of items we want to return. We fetch more than that
	// and then sort/slice locally because Spotify's ordering can be inconsistent
	// across include_groups/markets for some artists.
	want := limit
	if want <= 0 {
		want = 10
	}
	if want > 50 {
		want = 50
	}
	fetch := 50
	if want > fetch {
		fetch = want
	}

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

	byID := make(map[string]SpotifyAlbum, len(body.Items))
	for _, a := range body.Items {
		if a.ID == "" {
			continue
		}
		if existing, ok := byID[a.ID]; ok {
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

	sort.SliceStable(merged, func(i, j int) bool {
		di, okI := ParseSpotifyReleaseDate(merged[i].ReleaseDate)
		dj, okJ := ParseSpotifyReleaseDate(merged[j].ReleaseDate)

		if okI && okJ && !di.Equal(dj) {
			return di.After(dj)
		}
		if okI != okJ {
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

// ParseSpotifyReleaseDate parses Spotify `release_date` values which can be:
// - "YYYY-MM-DD"
// - "YYYY-MM"
// - "YYYY"
// Some APIs may return full timestamps; we accept RFC3339/RFC3339Nano too.
func ParseSpotifyReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}

	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse("2006-01", s); err == nil {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), true
	}
	if t, err := time.Parse("2006", s); err == nil {
		return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC), true
	}

	return time.Time{}, false
}
