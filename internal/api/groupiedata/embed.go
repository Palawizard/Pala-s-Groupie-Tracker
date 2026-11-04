// Package groupiedata ships a copy of the Groupie Trackers dataset.
//
// The public API (groupietrackers.herokuapp.com) and its source repository
// were taken down, so the artists and relations are bundled with the app.
// Artist images live in web/static/groupie/artists and the `image` field holds
// a root-relative path that callers prefix with the request base path.
package groupiedata

import _ "embed"

// ArtistsJSON is the original /api/artists payload (JSON array).
//
//go:embed artists.json
var ArtistsJSON []byte

// RelationJSON is the original /api/relation payload ({"index": [...]}).
//
//go:embed relation.json
var RelationJSON []byte
