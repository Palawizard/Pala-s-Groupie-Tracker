package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	artistsURL  = "https://groupietrackers.herokuapp.com/api/artists"
	relationURL = "https://groupietrackers.herokuapp.com/api/relation"
)

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

func FetchArtistByID(id int) (*Artist, error) {
	artists, err := FetchArtists()
	if err != nil {
		return nil, err
	}

	for _, a := range artists {
		if a.ID == id {
			return &a, nil
		}
	}

	return nil, errors.New("artist not found")
}

func FetchRelations() (*RelationIndex, error) {
	resp, err := http.Get(relationURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ri RelationIndex
	err = json.NewDecoder(resp.Body).Decode(&ri)
	if err != nil {
		return nil, err
	}

	return &ri, nil
}

func FetchRelationForArtist(id int) (*Relation, error) {
	ri, err := FetchRelations()
	if err != nil {
		return nil, err
	}

	for _, r := range ri.Index {
		if r.ID == id {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("relation not found for id %d", id)
}
