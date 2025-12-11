package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	wikiSummaryEndpoint = "https://en.wikipedia.org/api/rest_v1/page/summary/"
	wikiSearchEndpoint  = "https://en.wikipedia.org/w/api.php"
)

type wikiSummaryResponse struct {
	Extract     string `json:"extract"`
	ContentUrls struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

type wikiSearchResponse struct {
	Query struct {
		Search []struct {
			Title string `json:"title"`
		} `json:"search"`
	} `json:"query"`
}

func searchWikipediaTitle(rawQuery string) (string, error) {
	if rawQuery == "" {
		return "", fmt.Errorf("empty query")
	}

	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("format", "json")
	params.Set("utf8", "1")
	params.Set("srlimit", "5")
	params.Set("srsearch", rawQuery)

	u := wikiSearchEndpoint + "?" + params.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0 (contact@example.com)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search status: %s", resp.Status)
	}

	var payload wikiSearchResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return "", err
	}

	if len(payload.Query.Search) == 0 {
		return "", fmt.Errorf("no search results")
	}

	return payload.Query.Search[0].Title, nil
}

func FetchWikipediaSummary(title string) (string, string, error) {
	if title == "" {
		return "", "", fmt.Errorf("empty title")
	}

	var resolvedTitle string
	var err error

	resolvedTitle, err = searchWikipediaTitle(title + " band")
	if err != nil {
		resolvedTitle, err = searchWikipediaTitle(title + " music group")
	}
	if err != nil {
		resolvedTitle = title
	}

	escaped := url.PathEscape(resolvedTitle)
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
		return "", "", fmt.Errorf("summary status: %s", resp.Status)
	}

	var payload wikiSummaryResponse
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return "", "", err
	}

	if payload.Extract == "" || payload.ContentUrls.Desktop.Page == "" {
		return "", "", fmt.Errorf("missing summary data")
	}

	return payload.Extract, payload.ContentUrls.Desktop.Page, nil
}
