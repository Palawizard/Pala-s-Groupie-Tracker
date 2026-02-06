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
	BasePath  string
	Featured  []HomeArtistCard
}

// HomeHandler renders the homepage with a featured artists carousel
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)

	tmpl, err := template.ParseFiles(
		"web/templates/layout.gohtml",
		"web/templates/home.gohtml",
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	// Featured cards are source-specific (Groupie vs Spotify vs Deezer vs Apple)
	featured, err := buildHomeFeatured(basePath, source)
	if err != nil {
		http.Error(w, "failed to load home", http.StatusInternalServerError)
		return
	}

	data := HomePageData{
		Title:     "Groupie Tracker",
		Source:    source,
		ActiveNav: "home",
		BasePath:  basePath,
		Featured:  featured,
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

// buildHomeFeatured builds a small set of cards for the homepage marquee
func buildHomeFeatured(basePath, source string) ([]HomeArtistCard, error) {
	// Keep the marquee lightweight so the home page renders quickly
	desired := 24

	if source == "spotify" {
		// Use a broad query so this section always has results
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
				// First image is typically the largest in Spotify's response
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
				LinkURL:  basePath + "/artists/" + a.ID + "?source=spotify",
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
			// Prefer higher-res covers when available
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
				LinkURL:  basePath + "/artists/" + strconv.Itoa(a.ID) + "?source=deezer",
				Meta:     meta,
				Badge:    "Deezer",
			})
		}

		return out, nil
	}

	if source == "apple" {
		// Apple doesn't expose artist images directly, so we reuse recent album artwork
		artists, err := api.SearchAppleArtistsWithArtwork("a", desired, 300)
		if err != nil {
			return nil, err
		}

		limit := desired
		if len(artists) < limit {
			limit = len(artists)
		}

		out := make([]HomeArtistCard, 0, limit)
		for i := 0; i < limit; i++ {
			a := artists[i].Artist
			meta := "Apple artist"
			if a.PrimaryGenreName != "" {
				meta = a.PrimaryGenreName
			}

			out = append(out, HomeArtistCard{
				Name:     a.ArtistName,
				ImageURL: artists[i].ArtworkURL,
				LinkURL:  basePath + "/artists/" + strconv.Itoa(a.ArtistID) + "?source=apple",
				Meta:     meta,
				Badge:    "Apple",
			})
		}

		return out, nil
	}

	// Default "groupie" source uses the original dataset
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
			LinkURL:  basePath + "/artists/" + strconv.Itoa(a.ID) + "?source=groupie",
			// Keep metadata short so cards stay visually balanced
			Meta:  fmt.Sprintf("Created %d • %d members", a.CreationDate, len(a.Members)),
			Badge: "Groupie",
		})
	}

	return out, nil
}

// formatIntCompact formats large numbers as 1.2k, 3.4m, etc
func formatIntCompact(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n < 1000000 {
		v := float64(n) / 1000.0
		s := strconv.FormatFloat(v, 'f', 1, 64)
		// Avoid returning values like "1.0k"
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

// trimTrailingZero removes a trailing ".0" from a decimal string
func trimTrailingZero(s string) string {
	if len(s) >= 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}
