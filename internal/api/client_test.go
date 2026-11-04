package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedArtists(t *testing.T) {
	list, err := FetchArtists()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 52 {
		t.Fatalf("want 52 artists, got %d", len(list))
	}

	for _, a := range list {
		if a.Name == "" || len(a.Members) == 0 || a.CreationDate == 0 {
			t.Errorf("artist %d has missing fields", a.ID)
		}
		if !strings.HasPrefix(a.Image, "/static/groupie/artists/") {
			t.Errorf("artist %d image %q is not a local path", a.ID, a.Image)
			continue
		}
		// Images are served from web/static, two levels above this package
		file := filepath.Join("..", "..", "web", strings.TrimPrefix(a.Image, "/"))
		if _, err := os.Stat(file); err != nil {
			t.Errorf("artist %d image missing: %v", a.ID, err)
		}
	}

	queen, err := FetchArtistByID(1)
	if err != nil || queen.Name != "Queen" {
		t.Fatalf("artist 1: %v %+v", err, queen)
	}
}

func TestEmbeddedRelations(t *testing.T) {
	ri, err := FetchRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(ri.Index) != 52 {
		t.Fatalf("want 52 relations, got %d", len(ri.Index))
	}

	rel, err := FetchRelationForArtist(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := rel.DatesLocations["dunedin-new_zealand"]; len(got) != 1 || got[0] != "10-02-2020" {
		t.Fatalf("unexpected relation for Queen: %v", got)
	}
}
