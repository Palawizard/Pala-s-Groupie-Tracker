package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))

	var payload deezerListResponse[DeezerAlbum]
	if err := deezerGetJSON(deezerBaseURL+"/artist/"+strconv.Itoa(id)+"/albums?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	albums := payload.Data
	if len(albums) == 0 {
		return albums, nil
	}

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
