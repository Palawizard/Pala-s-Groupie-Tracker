package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"palasgroupietracker/internal/api"
	"palasgroupietracker/internal/store"
)

type FavoriteCard struct {
	Source   string
	ArtistID string
	Name     string
	ImageURL string
	LinkURL  string
	Meta     string
	Badge    string
}

type FavoritesPageData struct {
	Title      string
	Source     string
	ActiveNav  string
	BasePath   string
	CurrentURL string
	User       *store.User
	IsAuthed   bool

	Cards []FavoriteCard
}

// FavoritesHandler renders the favorites page for the current user
func FavoritesHandler(w http.ResponseWriter, r *http.Request) {
	basePath := getBasePath(r)

	user, authed := getCurrentUser(w, r)
	if !authed {
		http.Redirect(w, r, withBasePath(r, "/login")+"?next="+url.QueryEscape(buildCurrentURL(r)), http.StatusSeeOther)
		return
	}

	if appStore == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	favorites, err := appStore.ListFavorites(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to load favorites", http.StatusInternalServerError)
		return
	}

	cards, err := buildFavoriteCardsFromFavorites(basePath, favorites)
	if err != nil {
		http.Error(w, "failed to load favorites", http.StatusInternalServerError)
		return
	}

	data := FavoritesPageData{
		Title:      "Favorites",
		Source:     getSource(r),
		ActiveNav:  "favorites",
		BasePath:   basePath,
		CurrentURL: buildCurrentURL(r),
		User:       user,
		IsAuthed:   authed,
		Cards:      cards,
	}

	tmpl, err := templateWithLayout("web/templates/favorites.gohtml")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

// ToggleFavoriteHandler toggles a favorite for the current user
func ToggleFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, withBasePath(r, "/artists"), http.StatusSeeOther)
		return
	}

	source := normalizeSource(r.FormValue("source"))
	artistID := strings.TrimSpace(r.FormValue("artist_id"))
	redirectTo := resolveNextURL(r.FormValue("redirect"), r)

	if artistID == "" {
		http.Redirect(w, r, redirectTo, http.StatusSeeOther)
		return
	}

	user, authed := getCurrentUser(w, r)
	if !authed {
		http.Redirect(w, r, withBasePath(r, "/login")+"?next="+url.QueryEscape(redirectTo), http.StatusSeeOther)
		return
	}

	if appStore == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if _, err := appStore.ToggleFavorite(r.Context(), user.ID, source, artistID); err != nil {
		http.Error(w, "failed to update favorite", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

func buildFavoriteCardsFromFavorites(basePath string, favorites []store.Favorite) ([]FavoriteCard, error) {
	cards := make([]FavoriteCard, 0, len(favorites))

	for _, fav := range favorites {
		card, ok, err := buildFavoriteCard(basePath, fav.Source, fav.ArtistID)
		if err != nil {
			return nil, err
		}
		if ok {
			cards = append(cards, card)
		}
	}

	return cards, nil
}

func buildFavoriteCard(basePath, source, id string) (FavoriteCard, bool, error) {
	switch source {
	case "spotify":
		artist, err := api.GetSpotifyArtist(id)
		if err != nil || artist == nil {
			return FavoriteCard{}, false, nil
		}
		imageURL := ""
		if len(artist.Images) > 0 {
			imageURL = artist.Images[0].URL
		}
		meta := "Spotify artist"
		if artist.Followers != nil && artist.Followers.Total > 0 {
			meta = "Followers: " + strconv.Itoa(artist.Followers.Total)
		} else if len(artist.Genres) > 0 {
			meta = "Genre: " + artist.Genres[0]
		}
		return FavoriteCard{
			Source:   "spotify",
			ArtistID: id,
			Name:     artist.Name,
			ImageURL: imageURL,
			LinkURL:  basePath + "/artists/" + id + "?source=spotify",
			Meta:     meta,
			Badge:    "Spotify",
		}, true, nil
	case "deezer":
		intID, err := strconv.Atoi(id)
		if err != nil {
			return FavoriteCard{}, false, nil
		}
		artist, err := api.GetDeezerArtist(intID)
		if err != nil || artist == nil {
			return FavoriteCard{}, false, nil
		}
		imageURL := artist.PictureXL
		if imageURL == "" {
			imageURL = artist.PictureBig
		}
		if imageURL == "" {
			imageURL = artist.PictureMedium
		}
		meta := "Deezer artist"
		if artist.NbFan > 0 {
			meta = "Fans: " + strconv.Itoa(artist.NbFan)
		} else if artist.NbAlbum > 0 {
			meta = "Albums: " + strconv.Itoa(artist.NbAlbum)
		}
		return FavoriteCard{
			Source:   "deezer",
			ArtistID: id,
			Name:     artist.Name,
			ImageURL: imageURL,
			LinkURL:  basePath + "/artists/" + id + "?source=deezer",
			Meta:     meta,
			Badge:    "Deezer",
		}, true, nil
	case "apple":
		intID, err := strconv.Atoi(id)
		if err != nil {
			return FavoriteCard{}, false, nil
		}
		artist, err := api.GetAppleArtist(intID)
		if err != nil || artist == nil {
			return FavoriteCard{}, false, nil
		}
		artwork, _ := api.GetAppleArtistArtwork(intID, 300)
		meta := "Apple artist"
		if artist.PrimaryGenreName != "" {
			meta = "Genre: " + artist.PrimaryGenreName
		}
		return FavoriteCard{
			Source:   "apple",
			ArtistID: id,
			Name:     artist.ArtistName,
			ImageURL: artwork,
			LinkURL:  basePath + "/artists/" + id + "?source=apple",
			Meta:     meta,
			Badge:    "Apple",
		}, true, nil
	default:
		intID, err := strconv.Atoi(id)
		if err != nil {
			return FavoriteCard{}, false, nil
		}
		artist, err := api.FetchArtistByID(intID)
		if err != nil || artist == nil {
			return FavoriteCard{}, false, nil
		}
		meta := "Created " + strconv.Itoa(artist.CreationDate)
		return FavoriteCard{
			Source:   "groupie",
			ArtistID: id,
			Name:     artist.Name,
			ImageURL: artist.Image,
			LinkURL:  basePath + "/artists/" + id + "?source=groupie",
			Meta:     meta,
			Badge:    "Groupie",
		}, true, nil
	}
}

// favoriteIDMap returns a lookup map for favorite ids in the given source
func favoriteIDMap(r *http.Request, user *store.User, source string) map[string]bool {
	if user == nil || appStore == nil {
		return nil
	}

	ids, err := appStore.ListFavoriteIDsBySource(r.Context(), user.ID, source)
	if err != nil {
		return nil
	}

	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}

	return out
}

// isFavorite checks if the given artist is already saved
func isFavorite(r *http.Request, user *store.User, source, artistID string) bool {
	if user == nil || appStore == nil {
		return false
	}

	ok, err := appStore.IsFavorite(r.Context(), user.ID, source, artistID)
	if err != nil {
		return false
	}

	return ok
}

// note: we keep favorites order from the database (most recent first)
