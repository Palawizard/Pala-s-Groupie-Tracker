package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
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

func getSpotifyToken() (string, error) {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing spotify credentials")
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	if body.AccessToken == "" {
		return "", fmt.Errorf("empty spotify access token")
	}

	return body.AccessToken, nil
}

func SearchSpotifyArtists(query string) ([]SpotifyArtist, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	baseURL := "https://api.spotify.com/v1/search"
	params := url.Values{}
	if strings.TrimSpace(query) == "" {
		params.Set("q", "*")
	} else {
		params.Set("q", query)
	}
	params.Set("type", "artist")
	params.Set("limit", "30")
	params.Set("market", "US")

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify search failed: %s", resp.Status)
	}

	var body spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Artists.Items, nil
}

func GetSpotifyArtist(id string) (*SpotifyArtist, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	url := "https://api.spotify.com/v1/artists/" + id

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify artist failed: %s", resp.Status)
	}

	var artist SpotifyArtist
	if err := json.NewDecoder(resp.Body).Decode(&artist); err != nil {
		return nil, err
	}

	return &artist, nil
}

func GetSpotifyArtistTopTracks(id string, market string) ([]SpotifyTrack, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(market) == "" {
		market = "US"
	}

	baseURL := "https://api.spotify.com/v1/artists/" + id + "/top-tracks"
	params := url.Values{}
	params.Set("market", market)

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify top tracks failed: %s", resp.Status)
	}

	var body spotifyTopTracksResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Tracks, nil
}

func GetSpotifyArtistAlbums(id string, market string, limit int) ([]SpotifyAlbum, error) {
	token, err := getSpotifyToken()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(market) == "" {
		market = "US"
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	baseURL := "https://api.spotify.com/v1/artists/" + id + "/albums"
	params := url.Values{}
	params.Set("include_groups", "album,single")
	params.Set("market", market)
	params.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify artist albums failed: %s", resp.Status)
	}

	var body spotifyArtistAlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Items, nil
}
