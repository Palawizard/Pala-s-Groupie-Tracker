package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"palasgroupietracker/internal/api/groupiedata"
)

// The Groupie Trackers API is no longer online, so the dataset is embedded
// in the binary and decoded once on first use

var (
	artistsOnce sync.Once
	artists     []Artist
	artistsErr  error

	relationsOnce sync.Once
	relations     *RelationIndex
	relationsErr  error
)

// FetchArtists returns the full Groupie Trackers artist list
func FetchArtists() ([]Artist, error) {
	artistsOnce.Do(func() {
		var list []Artist
		if err := json.Unmarshal(groupiedata.ArtistsJSON, &list); err != nil {
			artistsErr = fmt.Errorf("decode embedded artists: %w", err)
			return
		}
		artists = list
	})
	if artistsErr != nil {
		return nil, artistsErr
	}
	// Callers may reorder the slice, so hand out a copy
	out := make([]Artist, len(artists))
	copy(out, artists)
	return out, nil
}

// FetchArtistByID returns the artist whose ID matches the given id
func FetchArtistByID(id int) (*Artist, error) {
	list, err := FetchArtists()
	if err != nil {
		return nil, err
	}

	for _, a := range list {
		if a.ID == id {
			// Return a pointer so templates can handle "missing" with nil checks
			return &a, nil
		}
	}

	return nil, errors.New("artist not found")
}

// FetchRelations returns the relations index (concert locations and dates)
func FetchRelations() (*RelationIndex, error) {
	relationsOnce.Do(func() {
		var ri RelationIndex
		if err := json.Unmarshal(groupiedata.RelationJSON, &ri); err != nil {
			relationsErr = fmt.Errorf("decode embedded relations: %w", err)
			return
		}
		relations = &ri
	})
	if relationsErr != nil {
		return nil, relationsErr
	}
	return relations, nil
}

// FetchRelationForArtist extracts the relation entry for a specific artist ID
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
