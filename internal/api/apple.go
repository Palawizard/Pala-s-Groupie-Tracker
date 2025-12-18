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

func appleDoJSON(u string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0")

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

func SearchAppleArtists(query string) ([]AppleArtist, error) {
	term := strings.TrimSpace(query)
	if term == "" {
		term = "a"
	}

	params := url.Values{}
	params.Set("term", term)
	params.Set("media", "music")
	params.Set("entity", "musicArtist")
	params.Set("limit", "30")
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
			continue
		}
		if a.ArtistID <= 0 || strings.TrimSpace(a.ArtistName) == "" {
			continue
		}
		out = append(out, a)
	}

	return out, nil
}

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
		artists = artists[:limit]
	}

	out := make([]AppleArtistWithArtwork, len(artists))
	for i := range artists {
		out[i].Artist = artists[i]
	}

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for i := range out {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			u, _ := GetAppleArtistArtwork(out[idx].Artist.ArtistID, artworkSize)
			out[idx].ArtworkURL = u
			<-sem
		}(i)
	}

	wg.Wait()
	return out, nil
}

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
			continue
		}
		if strings.ToLower(strings.TrimSpace(it.CollectionType)) != "album" && it.CollectionType != "" {
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

	sort.SliceStable(albums, func(i, j int) bool {
		di, okI := parseAppleDate(albums[i].ReleaseDate)
		dj, okJ := parseAppleDate(albums[j].ReleaseDate)
		if okI && okJ && !di.Equal(dj) {
			return di.After(dj)
		}
		if okI != okJ {
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

	sort.SliceStable(tracks, func(i, j int) bool {
		di, okI := parseAppleDate(tracks[i].ReleaseDate)
		dj, okJ := parseAppleDate(tracks[j].ReleaseDate)
		if okI && okJ && !di.Equal(dj) {
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
		art = upscaleAppleArtwork(normalizeAppleArtworkURL(it.ArtworkURL100), size)
		break
	}

	appleArtworkCache.mu.Lock()
	appleArtworkCache.m[artistID] = appleArtworkCacheItem{
		URL:       art,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	appleArtworkCache.mu.Unlock()

	return art, nil
}

func upscaleAppleArtwork(u string, size int) string {
	u = strings.TrimSpace(u)
	if u == "" || size <= 0 {
		return ""
	}

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
