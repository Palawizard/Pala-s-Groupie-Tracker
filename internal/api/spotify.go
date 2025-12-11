package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	spotifyTokenURL   = "https://accounts.spotify.com/api/token"
	spotifySearchURL  = "https://api.spotify.com/v1/search"
	spotifyArtistBase = "https://api.spotify.com/v1/artists"
	spotifyGrantType  = "client_credentials"
	spotifyAuthHeader = "application/x-www-form-urlencoded"
)

type spotifyTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type SpotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type SpotifyArtist struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Images       []SpotifyImage `json:"images"`
	Genres       []string       `json:"genres"`
	Popularity   int            `json:"popularity"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

type spotifySearchArtistsResponse struct {
	Artists struct {
		Items []SpotifyArtist `json:"items"`
	} `json:"artists"`
}

var (
	spotifyToken       string
	spotifyTokenExpiry time.Time
	spotifyMu          sync.Mutex
)

func getSpotifyToken() (string, error) {
	spotifyMu.Lock()
	defer spotifyMu.Unlock()

	if spotifyToken != "" && time.Now().Before(spotifyTokenExpiry) {
		return spotifyToken, nil
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Println("spotify: missing SPOTIFY_CLIENT_ID or SPOTIFY_CLIENT_SECRET")
		return "", errors.New("missing spotify client credentials")
	}

	data := url.Values{}
	data.Set("grant_type", spotifyGrantType)

	req, err := http.NewRequest("POST", spotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("spotify: failed to create token request: %v\n", err)
		return "", err
	}
	req.Header.Set("Content-Type", spotifyAuthHeader)

	encoded := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+encoded)

	log.Println("spotify: requesting new access token")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("spotify: token request error: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("spotify: token request failed: status=%d body=%s\n", resp.StatusCode, string(body))
		return "", errors.New("failed to get spotify token")
	}

	var payload spotifyTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		log.Printf("spotify: failed to decode token response: %v\n", err)
		return "", err
	}

	if payload.AccessToken == "" {
		log.Println("spotify: empty access token in response")
		return "", errors.New("empty spotify token")
	}

	spotifyToken = payload.AccessToken
	spotifyTokenExpiry = time.Now().Add(time.Duration(payload.ExpiresIn-30) * time.Second)
	log.Printf("spotify: acquired access token, expires in %d seconds\n", payload.ExpiresIn)

	return spotifyToken, nil
}

func SearchSpotifyArtists(query string) ([]SpotifyArtist, error) {
	rawQuery := strings.TrimSpace(query)
	if rawQuery == "" {
		rawQuery = "a"
	}

	log.Printf("spotify: searching artists for query=%q (raw input=%q)\n", rawQuery, query)
	token, err := getSpotifyToken()
	if err != nil {
		log.Printf("spotify: cannot search, token error: %v\n", err)
		return nil, err
	}

	params := url.Values{}
	params.Set("q", rawQuery)
	params.Set("type", "artist")
	params.Set("limit", "30")

	req, err := http.NewRequest("GET", spotifySearchURL+"?"+params.Encode(), nil)
	if err != nil {
		log.Printf("spotify: failed to create search request: %v\n", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("spotify: search request error: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("spotify: search request failed: status=%d body=%s\n", resp.StatusCode, string(body))
		return nil, errors.New("spotify search request failed")
	}

	var payload spotifySearchArtistsResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		log.Printf("spotify: failed to decode search response: %v\n", err)
		return nil, err
	}

	log.Printf("spotify: search returned %d artists\n", len(payload.Artists.Items))
	return payload.Artists.Items, nil
}

func GetSpotifyArtist(id string) (*SpotifyArtist, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		log.Println("spotify: empty artist id")
		return nil, errors.New("empty spotify artist id")
	}

	log.Printf("spotify: fetching artist id=%s\n", id)
	token, err := getSpotifyToken()
	if err != nil {
		log.Printf("spotify: cannot get artist, token error: %v\n", err)
		return nil, err
	}

	req, err := http.NewRequest("GET", spotifyArtistBase+"/"+id, nil)
	if err != nil {
		log.Printf("spotify: failed to create artist request: %v\n", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("spotify: artist request error: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("spotify: artist request failed: status=%d body=%s\n", resp.StatusCode, string(body))
		return nil, errors.New("spotify artist request failed")
	}

	var artist SpotifyArtist
	err = json.NewDecoder(resp.Body).Decode(&artist)
	if err != nil {
		log.Printf("spotify: failed to decode artist response: %v\n", err)
		return nil, err
	}

	log.Printf("spotify: fetched artist %s (%s)\n", artist.Name, artist.ID)
	return &artist, nil
}
