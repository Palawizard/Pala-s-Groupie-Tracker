package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const lastfmEndpoint = "https://ws.audioscrobbler.com/2.0/"

type lastfmArtistInfo struct {
	Artist struct {
		Stats struct {
			Listeners string `json:"listeners"`
		} `json:"stats"`
	} `json:"artist"`
}

func FetchArtistMonthlyListeners(artistName string) (int, error) {
	apiKey := os.Getenv("LASTFM_API_KEY")
	if apiKey == "" {
		return 0, errors.New("missing LASTFM_API_KEY")
	}

	name := strings.TrimSpace(artistName)
	if name == "" {
		return 0, errors.New("empty artist name")
	}

	params := url.Values{}
	params.Set("method", "artist.getInfo")
	params.Set("artist", name)
	params.Set("api_key", apiKey)
	params.Set("format", "json")

	u := lastfmEndpoint + "?" + params.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0 (contact@example.com)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.New("lastfm request failed")
	}

	var payload lastfmArtistInfo
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return 0, err
	}

	listenersStr := strings.TrimSpace(payload.Artist.Stats.Listeners)
	if listenersStr == "" {
		return 0, errors.New("no listeners in response")
	}

	value, err := strconv.Atoi(listenersStr)
	if err != nil {
		return 0, err
	}

	return value, nil
}
