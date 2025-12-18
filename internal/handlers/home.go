package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"palasgroupietracker/internal/api"
)

type HomeArtistCard struct {
	Name     string
	ImageURL string
	LinkURL  string
	Meta     string
	Badge    string
}

type HomePageData struct {
	Title     string
	Source    string
	ActiveNav string
	Featured  []HomeArtistCard
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/home.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	featured, err := buildHomeFeatured(source)
	if err != nil {
		http.Error(w, "failed to load home", http.StatusInternalServerError)
		return
	}

	data := HomePageData{
		Title:     "Groupie Tracker",
		Source:    source,
		ActiveNav: "home",
		Featured:  featured,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func buildHomeFeatured(source string) ([]HomeArtistCard, error) {
	desired := 24

	if source == "spotify" {
		artists, err := api.SearchSpotifyArtists("a")
		if err != nil {
			return nil, err
		}

		limit := desired
		if len(artists) < limit {
			limit = len(artists)
		}

		out := make([]HomeArtistCard, 0, limit)
		for i := 0; i < limit; i++ {
			a := artists[i]
			imageURL := ""
			if len(a.Images) > 0 {
				imageURL = a.Images[0].URL
			}

			meta := "Spotify artist"
			if a.Followers != nil && a.Followers.Total > 0 {
				meta = fmt.Sprintf("%s followers", formatIntCompact(a.Followers.Total))
			} else if len(a.Genres) > 0 {
				meta = a.Genres[0]
			}

			out = append(out, HomeArtistCard{
				Name:     a.Name,
				ImageURL: imageURL,
				LinkURL:  "/artists/" + a.ID + "?source=spotify",
				Meta:     meta,
				Badge:    "Spotify",
			})
		}

		return out, nil
	}

	if source == "deezer" {
		artists, err := api.SearchDeezerArtists("a")
		if err != nil {
			return nil, err
		}

		limit := desired
		if len(artists) < limit {
			limit = len(artists)
		}

		out := make([]HomeArtistCard, 0, limit)
		for i := 0; i < limit; i++ {
			a := artists[i]
			imageURL := a.PictureXL
			if imageURL == "" {
				imageURL = a.PictureBig
			}
			if imageURL == "" {
				imageURL = a.PictureMedium
			}
			if imageURL == "" {
				imageURL = a.Picture
			}

			meta := "Deezer artist"
			if a.NbFan > 0 {
				meta = fmt.Sprintf("%s fans", formatIntCompact(a.NbFan))
			} else if a.NbAlbum > 0 {
				meta = fmt.Sprintf("%d albums", a.NbAlbum)
			}

			out = append(out, HomeArtistCard{
				Name:     a.Name,
				ImageURL: imageURL,
				LinkURL:  "/artists/" + strconv.Itoa(a.ID) + "?source=deezer",
				Meta:     meta,
				Badge:    "Deezer",
			})
		}

		return out, nil
	}

	artists, err := api.FetchArtists()
	if err != nil {
		return nil, err
	}

	limit := desired
	if len(artists) < limit {
		limit = len(artists)
	}

	out := make([]HomeArtistCard, 0, limit)
	for i := 0; i < limit; i++ {
		a := artists[i]
		out = append(out, HomeArtistCard{
			Name:     a.Name,
			ImageURL: a.Image,
			LinkURL:  "/artists/" + strconv.Itoa(a.ID) + "?source=groupie",
			Meta:     fmt.Sprintf("Created %d • %d members", a.CreationDate, len(a.Members)),
			Badge:    "Groupie",
		})
	}

	return out, nil
}

func formatIntCompact(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n < 1000000 {
		v := float64(n) / 1000.0
		s := strconv.FormatFloat(v, 'f', 1, 64)
		s = trimTrailingZero(s)
		return s + "k"
	}
	if n < 1000000000 {
		v := float64(n) / 1000000.0
		s := strconv.FormatFloat(v, 'f', 1, 64)
		s = trimTrailingZero(s)
		return s + "m"
	}
	v := float64(n) / 1000000000.0
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = trimTrailingZero(s)
	return s + "b"
}

func trimTrailingZero(s string) string {
	if len(s) >= 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}
