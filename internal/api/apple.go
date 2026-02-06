package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const itunesBaseURL = "https://itunes.apple.com"

type AppleArtist struct {
	ArtistID         int    `json:"artistId"`
	ArtistName       string `json:"artistName"`
	PrimaryGenreName string `json:"primaryGenreName"`
	ArtistLinkURL    string `json:"artistLinkUrl"`
}

type AppleArtistWithArtwork struct {
	Artist     AppleArtist
	ArtworkURL string
}

type AppleAlbum struct {
	CollectionID      int    `json:"collectionId"`
	CollectionName    string `json:"collectionName"`
	CollectionType    string `json:"collectionType"`
	ReleaseDate       string `json:"releaseDate"`
	ArtworkURL100     string `json:"artworkUrl100"`
	CollectionViewURL string `json:"collectionViewUrl"`
	TrackCount        int    `json:"trackCount"`
	Country           string `json:"country"`
	Currency          string `json:"currency"`
}

type AppleTrack struct {
	TrackID         int    `json:"trackId"`
	TrackName       string `json:"trackName"`
	PreviewURL      string `json:"previewUrl"`
	TrackViewURL    string `json:"trackViewUrl"`
	TrackTimeMillis int    `json:"trackTimeMillis"`
	CollectionID    int    `json:"collectionId"`
	CollectionName  string `json:"collectionName"`
	ArtworkURL100   string `json:"artworkUrl100"`
	ReleaseDate     string `json:"releaseDate"`
}

type appleSearchResponse struct {
	ResultCount int               `json:"resultCount"`
	Results     []json.RawMessage `json:"results"`
}

type appleLookupItem struct {
	WrapperType      string `json:"wrapperType"`
	Kind             string `json:"kind"`
	ArtistID         int    `json:"artistId"`
	ArtistName       string `json:"artistName"`
	ArtistLinkURL    string `json:"artistLinkUrl"`
	PrimaryGenreName string `json:"primaryGenreName"`

	CollectionID      int    `json:"collectionId"`
	CollectionName    string `json:"collectionName"`
	CollectionType    string `json:"collectionType"`
	CollectionViewURL string `json:"collectionViewUrl"`
	TrackCount        int    `json:"trackCount"`

	TrackID         int    `json:"trackId"`
	TrackName       string `json:"trackName"`
	PreviewURL      string `json:"previewUrl"`
	TrackViewURL    string `json:"trackViewUrl"`
	TrackTimeMillis int    `json:"trackTimeMillis"`

	ArtworkURL100 string `json:"artworkUrl100"`
	ReleaseDate   string `json:"releaseDate"`
	Country       string `json:"country"`
	Currency      string `json:"currency"`
}

type appleArtworkCacheItem struct {
	URL       string
	ExpiresAt time.Time
}

var appleArtworkCache = struct {
	mu sync.RWMutex
	m  map[int]appleArtworkCacheItem
}{
	m: make(map[int]appleArtworkCacheItem),
}

// appleDoJSON performs a GET request to iTunes and decodes the JSON response into out
func appleDoJSON(u string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0")

	// Keep a short timeout so the UI doesn't hang on external APIs
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("itunes request failed: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// SearchAppleArtists searches iTunes for music artists matching query
func SearchAppleArtists(query string) ([]AppleArtist, error) {
	term := strings.TrimSpace(query)
	if term == "" {
		// iTunes rejects empty terms, so use a cheap fallback
		term = "a"
	}

	params := url.Values{}
	params.Set("term", term)
	params.Set("media", "music")
	params.Set("entity", "musicArtist")
	params.Set("limit", "30")
	// Using a fixed market keeps results stable for the project
	params.Set("country", "FR")
	params.Set("lang", "en_us")

	var payload appleSearchResponse
	if err := appleDoJSON(itunesBaseURL+"/search?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	out := make([]AppleArtist, 0, payload.ResultCount)
	for _, raw := range payload.Results {
		var a AppleArtist
		if err := json.Unmarshal(raw, &a); err != nil {
			// iTunes mixes items sometimes, skip anything we can't decode
			continue
		}
		if a.ArtistID <= 0 || strings.TrimSpace(a.ArtistName) == "" {
			// Filter out incomplete hits to avoid broken links in the UI
			continue
		}
		out = append(out, a)
	}

	return out, nil
}

// SearchAppleArtistsWithArtwork returns artists plus a "best effort" artwork URL for each artist
func SearchAppleArtistsWithArtwork(query string, limit int, artworkSize int) ([]AppleArtistWithArtwork, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	if artworkSize <= 0 {
		artworkSize = 300
	}

	artists, err := SearchAppleArtists(query)
	if err != nil {
		return nil, err
	}

	if len(artists) > limit {
		// Keep the UI snappy and avoid making too many downstream calls
		artists = artists[:limit]
	}

	out := make([]AppleArtistWithArtwork, len(artists))
	for i := range artists {
		out[i].Artist = artists[i]
	}

	// Limit concurrent lookups so we don't hammer iTunes and get throttled
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for i := range out {
		wg.Add(1)
		go func(idx int) { // fetch artwork concurrently with a small cap
			defer wg.Done()
			sem <- struct{}{}
			// Artwork is optional, ignore errors and keep the artist entry
			u, _ := GetAppleArtistArtwork(out[idx].Artist.ArtistID, artworkSize)
			out[idx].ArtworkURL = u
			<-sem
		}(i)
	}

	wg.Wait()
	return out, nil
}

// GetAppleArtist fetches basic artist info by iTunes artist ID
func GetAppleArtist(id int) (*AppleArtist, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid apple artist id")
	}

	params := url.Values{}
	params.Set("id", strconv.Itoa(id))

	var payload appleSearchResponse
	if err := appleDoJSON(itunesBaseURL+"/lookup?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	for _, raw := range payload.Results {
		var it appleLookupItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		if it.ArtistID == id && it.ArtistName != "" {
			return &AppleArtist{
				ArtistID:         it.ArtistID,
				ArtistName:       it.ArtistName,
				PrimaryGenreName: it.PrimaryGenreName,
				ArtistLinkURL:    it.ArtistLinkURL,
			}, nil
		}
	}

	return nil, fmt.Errorf("apple artist not found")
}

// GetAppleArtistAlbums returns the latest albums for an artist using iTunes lookup
func GetAppleArtistAlbums(artistID int, limit int) ([]AppleAlbum, error) {
	if artistID <= 0 {
		return nil, fmt.Errorf("invalid apple artist id")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	params := url.Values{}
	params.Set("id", strconv.Itoa(artistID))
	params.Set("entity", "album")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sort", "recent")
	params.Set("country", "FR")

	var payload appleSearchResponse
	if err := appleDoJSON(itunesBaseURL+"/lookup?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	var albums []AppleAlbum
	for _, raw := range payload.Results {
		var it appleLookupItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		if it.WrapperType != "collection" {
			// Skip non-album items like the artist wrapper
			continue
		}
		if strings.ToLower(strings.TrimSpace(it.CollectionType)) != "album" && it.CollectionType != "" {
			// iTunes also returns "single" here sometimes, ignore those
			continue
		}
		if it.CollectionID <= 0 || strings.TrimSpace(it.CollectionName) == "" {
			continue
		}

		albums = append(albums, AppleAlbum{
			CollectionID:      it.CollectionID,
			CollectionName:    it.CollectionName,
			CollectionType:    it.CollectionType,
			ReleaseDate:       it.ReleaseDate,
			ArtworkURL100:     normalizeAppleArtworkURL(it.ArtworkURL100),
			CollectionViewURL: it.CollectionViewURL,
			TrackCount:        it.TrackCount,
			Country:           it.Country,
			Currency:          it.Currency,
		})
	}

	sort.SliceStable(albums, func(i, j int) bool { // newest first, then stable tie-breakers
		di, okI := parseAppleDate(albums[i].ReleaseDate)
		dj, okJ := parseAppleDate(albums[j].ReleaseDate)
		if okI && okJ && !di.Equal(dj) {
			// Prefer newest releases when we can parse both dates
			return di.After(dj)
		}
		if okI != okJ {
			// Prefer entries with a parseable date
			return okI
		}
		ni := strings.ToLower(albums[i].CollectionName)
		nj := strings.ToLower(albums[j].CollectionName)
		if ni != nj {
			return ni < nj
		}
		return albums[i].CollectionID < albums[j].CollectionID
	})

	if len(albums) > limit {
		albums = albums[:limit]
	}

	return albums, nil
}

// GetAppleArtistSongs returns recent songs for an artist using iTunes lookup
func GetAppleArtistSongs(artistID int, limit int) ([]AppleTrack, error) {
	if artistID <= 0 {
		return nil, fmt.Errorf("invalid apple artist id")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	params := url.Values{}
	params.Set("id", strconv.Itoa(artistID))
	params.Set("entity", "song")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sort", "recent")
	params.Set("country", "FR")

	var payload appleSearchResponse
	if err := appleDoJSON(itunesBaseURL+"/lookup?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	var tracks []AppleTrack
	for _, raw := range payload.Results {
		var it appleLookupItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		if it.WrapperType != "track" || it.Kind != "song" {
			// Filter out non-song track types
			continue
		}
		if it.TrackID <= 0 || strings.TrimSpace(it.TrackName) == "" {
			continue
		}

		tracks = append(tracks, AppleTrack{
			TrackID:         it.TrackID,
			TrackName:       it.TrackName,
			PreviewURL:      it.PreviewURL,
			TrackViewURL:    it.TrackViewURL,
			TrackTimeMillis: it.TrackTimeMillis,
			CollectionID:    it.CollectionID,
			CollectionName:  it.CollectionName,
			ArtworkURL100:   normalizeAppleArtworkURL(it.ArtworkURL100),
			ReleaseDate:     it.ReleaseDate,
		})
	}

	sort.SliceStable(tracks, func(i, j int) bool { // newest first, then stable tie-breakers
		di, okI := parseAppleDate(tracks[i].ReleaseDate)
		dj, okJ := parseAppleDate(tracks[j].ReleaseDate)
		if okI && okJ && !di.Equal(dj) {
			// Keep the newest track first when possible
			return di.After(dj)
		}
		if okI != okJ {
			return okI
		}
		ni := strings.ToLower(tracks[i].TrackName)
		nj := strings.ToLower(tracks[j].TrackName)
		if ni != nj {
			return ni < nj
		}
		return tracks[i].TrackID < tracks[j].TrackID
	})

	if len(tracks) > limit {
		tracks = tracks[:limit]
	}

	return tracks, nil
}

// GetAppleArtistArtwork tries to find a representative image by looking up the artist's latest album
func GetAppleArtistArtwork(artistID int, size int) (string, error) {
	if artistID <= 0 {
		return "", fmt.Errorf("invalid apple artist id")
	}
	if size <= 0 {
		size = 300
	}

	now := time.Now()
	appleArtworkCache.mu.RLock()
	if it, ok := appleArtworkCache.m[artistID]; ok && it.URL != "" && now.Before(it.ExpiresAt) {
		appleArtworkCache.mu.RUnlock()
		// Cache hits are common on list pages, keep this path cheap
		return it.URL, nil
	}
	appleArtworkCache.mu.RUnlock()

	params := url.Values{}
	params.Set("id", strconv.Itoa(artistID))
	params.Set("entity", "album")
	params.Set("limit", "1")
	params.Set("sort", "recent")
	params.Set("country", "FR")

	var payload appleSearchResponse
	if err := appleDoJSON(itunesBaseURL+"/lookup?"+params.Encode(), &payload); err != nil {
		return "", err
	}

	art := ""
	for _, raw := range payload.Results {
		var it appleLookupItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		if it.WrapperType != "collection" {
			continue
		}
		if it.ArtworkURL100 == "" {
			continue
		}
		// iTunes images are size-encoded in the last path segment
		art = upscaleAppleArtwork(normalizeAppleArtworkURL(it.ArtworkURL100), size)
		break
	}

	appleArtworkCache.mu.Lock()
	// Cache empty strings too to avoid repeated lookups on missing artwork
	appleArtworkCache.m[artistID] = appleArtworkCacheItem{
		URL:       art,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	appleArtworkCache.mu.Unlock()

	return art, nil
}

// upscaleAppleArtwork rewrites iTunes artwork URLs to request a larger square image
func upscaleAppleArtwork(u string, size int) string {
	u = strings.TrimSpace(u)
	if u == "" || size <= 0 {
		return ""
	}

	// Typical tail is like `100x100bb.jpg`, rewrite it to `${size}x${size}bb.<ext>`
	parts := strings.Split(u, "/")
	if len(parts) == 0 {
		return u
	}

	last := parts[len(parts)-1]
	x := strings.Index(last, "x")
	bb := strings.Index(last, "bb.")
	if x > 0 && bb > x {
		ext := last[bb+3:]
		if ext != "" {
			parts[len(parts)-1] = strconv.Itoa(size) + "x" + strconv.Itoa(size) + "bb." + ext
			return strings.Join(parts, "/")
		}
	}

	return u
}

// normalizeAppleArtworkURL upgrades http links to https and trims whitespace
func normalizeAppleArtworkURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http://") {
		return "https://" + strings.TrimPrefix(u, "http://")
	}
	return u
}

// parseAppleDate tries common iTunes date formats
func parseAppleDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}
