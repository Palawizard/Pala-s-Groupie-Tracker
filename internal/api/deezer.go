package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const deezerBaseURL = "https://api.deezer.com"

type DeezerAPIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type deezerErrorEnvelope struct {
	Error *DeezerAPIError `json:"error"`
}

type DeezerArtist struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Link          string `json:"link"`
	Share         string `json:"share"`
	Picture       string `json:"picture"`
	PictureSmall  string `json:"picture_small"`
	PictureMedium string `json:"picture_medium"`
	PictureBig    string `json:"picture_big"`
	PictureXL     string `json:"picture_xl"`
	NbAlbum       int    `json:"nb_album"`
	NbFan         int    `json:"nb_fan"`
	Radio         bool   `json:"radio"`
	Tracklist     string `json:"tracklist"`
}

type DeezerAlbum struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	RecordType     string `json:"record_type"`
	Link           string `json:"link"`
	Share          string `json:"share"`
	Cover          string `json:"cover"`
	CoverSmall     string `json:"cover_small"`
	CoverMedium    string `json:"cover_medium"`
	CoverBig       string `json:"cover_big"`
	CoverXL        string `json:"cover_xl"`
	ReleaseDate    string `json:"release_date"`
	NbTracks       int    `json:"nb_tracks"`
	Fans           int    `json:"fans"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Tracklist      string `json:"tracklist"`
}

type DeezerTrack struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	Preview        string `json:"preview"`
	Duration       int    `json:"duration"`
	Rank           int    `json:"rank"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Album          struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Link        string `json:"link"`
		Cover       string `json:"cover"`
		CoverSmall  string `json:"cover_small"`
		CoverMedium string `json:"cover_medium"`
		CoverBig    string `json:"cover_big"`
		CoverXL     string `json:"cover_xl"`
	} `json:"album"`
}

type deezerListResponse[T any] struct {
	Data  []T    `json:"data"`
	Total int    `json:"total"`
	Next  string `json:"next"`
}

func deezerGetJSON(fullURL string, out any) error {
	req, err := http.NewRequest("GET", fullURL, nil)
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deezer request failed: %s", resp.Status)
	}

	var env deezerErrorEnvelope
	_ = json.Unmarshal(b, &env)
	if env.Error != nil {
		msg := strings.TrimSpace(env.Error.Message)
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("deezer error %d: %s", env.Error.Code, msg)
	}

	if err := json.Unmarshal(b, out); err != nil {
		return err
	}

	return nil
}

func SearchDeezerArtists(query string) ([]DeezerArtist, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = "a"
	}

	params := url.Values{}
	params.Set("q", q)

	var payload deezerListResponse[DeezerArtist]
	if err := deezerGetJSON(deezerBaseURL+"/search/artist?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	return payload.Data, nil
}

func GetDeezerArtist(id int) (*DeezerArtist, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid deezer artist id")
	}

	var artist DeezerArtist
	if err := deezerGetJSON(deezerBaseURL+"/artist/"+strconv.Itoa(id), &artist); err != nil {
		return nil, err
	}

	if artist.ID == 0 {
		return nil, fmt.Errorf("deezer artist not found")
	}

	return &artist, nil
}

func GetDeezerArtistTopTracks(id int, limit int) ([]DeezerTrack, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid deezer artist id")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))

	var payload deezerListResponse[DeezerTrack]
	if err := deezerGetJSON(deezerBaseURL+"/artist/"+strconv.Itoa(id)+"/top?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	return payload.Data, nil
}

func GetDeezerArtistAlbums(id int, limit int) ([]DeezerAlbum, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid deezer artist id")
	}
	want := limit
	if want <= 0 {
		want = 10
	}
	if want > 50 {
		want = 50
	}

	// Deezer can expose albums vs singles/EP via a `type` filter depending on the artist.
	// To keep the "Latest releases" section consistent, try multiple types and merge.
	fetch := 50
	recordTypes := []string{"", "single", "ep", "album"}

	ordered := make([]DeezerAlbum, 0, fetch)
	seen := make(map[int]int, fetch)
	for _, rt := range recordTypes {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(fetch))
		if strings.TrimSpace(rt) != "" {
			params.Set("type", rt)
		}

		var payload deezerListResponse[DeezerAlbum]
		if err := deezerGetJSON(deezerBaseURL+"/artist/"+strconv.Itoa(id)+"/albums?"+params.Encode(), &payload); err != nil {
			// The unfiltered request is required; typed requests are best-effort.
			if strings.TrimSpace(rt) == "" {
				return nil, err
			}
			continue
		}

		for _, a := range payload.Data {
			if a.ID <= 0 {
				continue
			}
			if idx, ok := seen[a.ID]; ok {
				if ordered[idx].ReleaseDate == "" && a.ReleaseDate != "" {
					ordered[idx].ReleaseDate = a.ReleaseDate
				}
				if ordered[idx].RecordType == "" && a.RecordType != "" {
					ordered[idx].RecordType = a.RecordType
				}
				continue
			}
			seen[a.ID] = len(ordered)
			ordered = append(ordered, a)
		}
	}

	if len(ordered) == 0 {
		return ordered, nil
	}

	candidateCount := want * 6
	if candidateCount < 30 {
		candidateCount = 30
	}
	if candidateCount > 50 {
		candidateCount = 50
	}
	if candidateCount > len(ordered) {
		candidateCount = len(ordered)
	}
	if candidateCount < want {
		candidateCount = want
		if candidateCount > len(ordered) {
			candidateCount = len(ordered)
		}
	}

	albums := ordered[:candidateCount]

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for i := range albums {
		wg.Add(1)
		go func(a *DeezerAlbum) {
			defer wg.Done()
			sem <- struct{}{}
			full, err := GetDeezerAlbum(a.ID)
			if err == nil && full != nil {
				a.ReleaseDate = full.ReleaseDate
				if a.RecordType == "" {
					a.RecordType = full.RecordType
				}
				a.NbTracks = full.NbTracks
				a.Fans = full.Fans
				a.ExplicitLyrics = full.ExplicitLyrics
				a.Tracklist = full.Tracklist
				if a.Link == "" {
					a.Link = full.Link
				}
				if a.Share == "" {
					a.Share = full.Share
				}
				if a.CoverXL == "" {
					a.CoverXL = full.CoverXL
				}
				if a.CoverBig == "" {
					a.CoverBig = full.CoverBig
				}
				if a.CoverMedium == "" {
					a.CoverMedium = full.CoverMedium
				}
				if a.CoverSmall == "" {
					a.CoverSmall = full.CoverSmall
				}
				if a.Cover == "" {
					a.Cover = full.Cover
				}
				if a.Title == "" {
					a.Title = full.Title
				}
			}
			<-sem
		}(&albums[i])
	}

	wg.Wait()

	sort.SliceStable(albums, func(i, j int) bool {
		di, okI := ParseDeezerReleaseDate(albums[i].ReleaseDate)
		dj, okJ := ParseDeezerReleaseDate(albums[j].ReleaseDate)

		if okI && okJ && !di.Equal(dj) {
			return di.After(dj)
		}
		if okI != okJ {
			return okI
		}

		ti := strings.ToLower(albums[i].Title)
		tj := strings.ToLower(albums[j].Title)
		if ti != tj {
			return ti < tj
		}

		return albums[i].ID < albums[j].ID
	})

	if len(albums) > want {
		albums = albums[:want]
	}

	return albums, nil
}

func GetDeezerAlbum(id int) (*DeezerAlbum, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid deezer album id")
	}

	var album DeezerAlbum
	if err := deezerGetJSON(deezerBaseURL+"/album/"+strconv.Itoa(id), &album); err != nil {
		return nil, err
	}

	if album.ID == 0 {
		return nil, fmt.Errorf("deezer album not found")
	}

	return &album, nil
}

// ParseDeezerReleaseDate parses Deezer `release_date` values.
// Typically it's "YYYY-MM-DD", but be tolerant of RFC3339 timestamps too.
func ParseDeezerReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000-00-00" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	// Some APIs occasionally return timestamps; be tolerant.
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
	return time.Time{}, false
}
