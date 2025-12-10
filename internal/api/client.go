package api

import (
	"encoding/json"
	"net/http"
)

const artistsURL = "https://groupietrackers.herokuapp.com/api/artists"

func FetchArtists() ([]Artist, error) {
	resp, err := http.Get(artistsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var artists []Artist
	err = json.NewDecoder(resp.Body).Decode(&artists)
	if err != nil {
		return nil, err
	}

	return artists, nil
}
