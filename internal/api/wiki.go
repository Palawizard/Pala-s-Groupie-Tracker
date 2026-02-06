package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// searchWikipediaTitle runs a search query and tries to pick the most suitable page title
func searchWikipediaTitle(rawQuery string) (string, error) {
	if rawQuery == "" {
		return "", fmt.Errorf("empty query")
	}

	base := strings.TrimSpace(rawQuery)
	lowerBase := strings.ToLower(base)
	// Strip common disambiguation suffixes to increase the chance of an exact match
	suffixes := []string{" band", " music group", " musical group", " singer", " musician", " rapper", " artist"}
	for _, s := range suffixes {
		if strings.HasSuffix(lowerBase, s) {
			base = strings.TrimSpace(base[:len(base)-len(s)])
			break
		}
	}
	lowerBase = strings.ToLower(base)

	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("format", "json")
	params.Set("utf8", "1")
	params.Set("srlimit", "10")
	params.Set("srsearch", rawQuery)

	u := wikiSearchEndpoint + "?" + params.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}

	// Wikipedia recommends setting a descriptive UA
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

	// Prefer exact title matches, then common disambiguation variants, then first hit
	exactTitle := ""
	prefTitle := ""
	anyTitle := ""

	for _, hit := range payload.Query.Search {
		title := hit.Title
		lower := strings.ToLower(title)

		if lower == lowerBase {
			exactTitle = title
			break
		}

		if strings.HasPrefix(lower, lowerBase+" (") {
			if strings.Contains(lower, "(band)") ||
				strings.Contains(lower, "(music group)") ||
				strings.Contains(lower, "(musical group)") ||
				strings.Contains(lower, "(singer)") ||
				strings.Contains(lower, "(musician)") ||
				strings.Contains(lower, "(rapper)") ||
				strings.Contains(lower, "(artist)") {
				if prefTitle == "" {
					prefTitle = title
				}
			}
		}

		if anyTitle == "" {
			anyTitle = title
		}
	}

	if exactTitle != "" {
		return exactTitle, nil
	}
	if prefTitle != "" {
		return prefTitle, nil
	}
	if anyTitle != "" {
		return anyTitle, nil
	}

	return "", fmt.Errorf("no suitable title")
}

// FetchWikipediaSummary returns the page summary extract and desktop URL for the best matching title
func FetchWikipediaSummary(title string) (string, string, error) {
	if title == "" {
		return "", "", fmt.Errorf("empty title")
	}

	var resolvedTitle string
	var err error

	// Try a few targeted queries first to avoid people/places with similar names
	resolvedTitle, err = searchWikipediaTitle(title + " artist")
	if err != nil {
		resolvedTitle, err = searchWikipediaTitle(title + " band")
	}
	if err != nil {
		resolvedTitle, err = searchWikipediaTitle(title + " music group")
	}
	if err != nil {
		resolvedTitle, err = searchWikipediaTitle(title)
	}
	if err != nil {
		return "", "", err
	}

	// Summary endpoint uses the page title as a path segment
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
