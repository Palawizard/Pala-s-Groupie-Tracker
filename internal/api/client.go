package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	artistsURL  = "https://groupietrackers.herokuapp.com/api/artists"
	relationURL = "https://groupietrackers.herokuapp.com/api/relation"
)

const groupieCacheTTL = 10 * time.Minute

var (
	artistsCacheMu      sync.Mutex
	artistsCacheFetched time.Time
	artistsCache        []Artist

	relationsCacheMu      sync.Mutex
	relationsCacheFetched time.Time
	relationsCache        *RelationIndex
)

func cacheFresh(since time.Time) bool {
	return !since.IsZero() && time.Since(since) < groupieCacheTTL
}

// FetchArtists loads the full artist list from the Groupie Trackers API
func FetchArtists() ([]Artist, error) {
	artistsCacheMu.Lock()
	if cacheFresh(artistsCacheFetched) && len(artistsCache) > 0 {
		cached := artistsCache
		artistsCacheMu.Unlock()
		return cached, nil
	}
	stale := artistsCache
	artistsCacheMu.Unlock()

	// Default client is fine here, this is a school project and the endpoint is public
	resp, err := http.Get(artistsURL)
	if err != nil {
		if len(stale) > 0 {
			// Best-effort: keep the UI functional if the API is temporarily unreachable
			return stale, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	var artists []Artist
	// The endpoint returns a JSON array of artists
	err = json.NewDecoder(resp.Body).Decode(&artists)
	if err != nil {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, err
	}

	artistsCacheMu.Lock()
	artistsCache = artists
	artistsCacheFetched = time.Now()
	artistsCacheMu.Unlock()

	return artists, nil
}

// FetchArtistByID returns the artist whose ID matches the given id
func FetchArtistByID(id int) (*Artist, error) {
	// The API doesn't provide a single-artist endpoint, so we filter client-side
	artists, err := FetchArtists()
	if err != nil {
		return nil, err
	}

	for _, a := range artists {
		if a.ID == id {
			// Return a pointer so templates can handle "missing" with nil checks
			return &a, nil
		}
	}

	return nil, errors.New("artist not found")
}

// FetchRelations loads the full relations index from the Groupie Trackers API
func FetchRelations() (*RelationIndex, error) {
	relationsCacheMu.Lock()
	if cacheFresh(relationsCacheFetched) && relationsCache != nil {
		cached := relationsCache
		relationsCacheMu.Unlock()
		return cached, nil
	}
	stale := relationsCache
	relationsCacheMu.Unlock()

	resp, err := http.Get(relationURL)
	if err != nil {
		if stale != nil && len(stale.Index) > 0 {
			return stale, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	var ri RelationIndex
	// The endpoint returns an object with an `index` array
	err = json.NewDecoder(resp.Body).Decode(&ri)
	if err != nil {
		if stale != nil && len(stale.Index) > 0 {
			return stale, nil
		}
		return nil, err
	}

	relationsCacheMu.Lock()
	relationsCache = &ri
	relationsCacheFetched = time.Now()
	relationsCacheMu.Unlock()

	return &ri, nil
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
