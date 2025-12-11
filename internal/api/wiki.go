package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const wikiSummaryEndpoint = "https://en.wikipedia.org/api/rest_v1/page/summary/"

type wikiSummaryResponse struct {
	Extract     string `json:"extract"`
	ContentUrls struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

func FetchWikipediaSummary(title string) (string, string, error) {
	if title == "" {
		return "", "", fmt.Errorf("empty title")
	}

	escaped := url.PathEscape(title)
	fullURL := wikiSummaryEndpoint + escaped

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0 (contact@example.com)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload wikiSummaryResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return "", "", err
	}

	if payload.Extract == "" || payload.ContentUrls.Desktop.Page == "" {
		return "", "", fmt.Errorf("missing data")
	}

	return payload.Extract, payload.ContentUrls.Desktop.Page, nil
}
