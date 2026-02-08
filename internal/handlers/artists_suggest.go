package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"palasgroupietracker/internal/api"
	"palasgroupietracker/internal/geo"
)

type Suggestion struct {
	Type   string `json:"type"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Target string `json:"target"` // which input should be filled (q/location)
}

type suggestItem struct {
	Suggestion
	norm string
}

var (
	suggestCacheMu      sync.Mutex
	suggestCacheFetched time.Time
	suggestCacheItems   []suggestItem
)

const suggestCacheTTL = 10 * time.Minute

// ArtistsSuggestHandler returns search suggestions for the artists page.
// It is intentionally limited to Groupie mode to keep it deterministic and fast.
func ArtistsSuggestHandler(w http.ResponseWriter, r *http.Request) {
	if getSource(r) != "groupie" {
		writeJSON(w, http.StatusOK, []Suggestion{})
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(raw)) < 2 {
		writeJSON(w, http.StatusOK, []Suggestion{})
		return
	}
	q := normalizeForMatch(raw)
	if q == "" {
		writeJSON(w, http.StatusOK, []Suggestion{})
		return
	}

	items, err := getGroupieSuggestItems()
	if err != nil {
		http.Error(w, "failed to build suggestions", http.StatusInternalServerError)
		return
	}

	type scored struct {
		item  suggestItem
		score int
	}

	// Lower score is better.
	matches := make([]scored, 0, 16)
	for _, it := range items {
		if it.norm == "" {
			continue
		}
		if !strings.Contains(it.norm, q) {
			continue
		}
		score := 2
		if strings.HasPrefix(it.norm, q) {
			score = 0
		} else if strings.Contains(it.norm, " "+q) {
			score = 1
		}
		matches = append(matches, scored{item: it, score: score})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		if matches[i].item.Type != matches[j].item.Type {
			// Keep consistent ordering: group -> member -> location.
			order := func(t string) int {
				switch t {
				case "group":
					return 0
				case "member":
					return 1
				case "location":
					return 2
				default:
					return 3
				}
			}
			return order(matches[i].item.Type) < order(matches[j].item.Type)
		}
		return strings.ToLower(matches[i].item.Label) < strings.ToLower(matches[j].item.Label)
	})

	out := make([]Suggestion, 0, 10)
	seen := make(map[string]struct{}, 16)
	for _, m := range matches {
		k := m.item.Type + "\x00" + strings.ToLower(m.item.Label)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, m.item.Suggestion)
		if len(out) >= 10 {
			break
		}
	}

	writeJSON(w, http.StatusOK, out)
}

func getGroupieSuggestItems() ([]suggestItem, error) {
	suggestCacheMu.Lock()
	if !suggestCacheFetched.IsZero() && time.Since(suggestCacheFetched) < suggestCacheTTL && len(suggestCacheItems) > 0 {
		cached := suggestCacheItems
		suggestCacheMu.Unlock()
		return cached, nil
	}
	suggestCacheMu.Unlock()

	artists, err := api.FetchArtists()
	if err != nil {
		return nil, err
	}
	relations, err := api.FetchRelations()
	if err != nil {
		return nil, err
	}

	byKey := make(map[string]suggestItem, 1024)
	add := func(t, label, value, target string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		target = strings.TrimSpace(target)
		if label == "" || value == "" || target == "" {
			return
		}
		it := suggestItem{
			Suggestion: Suggestion{
				Type:   t,
				Label:  label,
				Value:  value,
				Target: target,
			},
			norm: normalizeForMatch(label),
		}
		if it.norm == "" {
			return
		}
		k := t + "\x00" + it.norm
		if _, ok := byKey[k]; ok {
			return
		}
		byKey[k] = it
	}

	for _, a := range artists {
		add("group", a.Name, a.Name, "q")
		for _, m := range a.Members {
			add("member", m, m, "q")
		}
	}

	for _, rel := range relations.Index {
		for key := range rel.DatesLocations {
			_, _, display := geo.QueryFromLocationKey(key)
			// Use the raw key as a fallback so locations are still discoverable.
			if strings.TrimSpace(display) == "" {
				display = key
			}
			add("location", display, display, "location")
		}
	}

	items := make([]suggestItem, 0, len(byKey))
	for _, it := range byKey {
		items = append(items, it)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			if items[i].Type == "group" {
				return true
			}
			if items[j].Type == "group" {
				return false
			}
			if items[i].Type == "member" {
				return true
			}
			if items[j].Type == "member" {
				return false
			}
		}
		return strings.ToLower(items[i].Label) < strings.ToLower(items[j].Label)
	})

	suggestCacheMu.Lock()
	suggestCacheItems = items
	suggestCacheFetched = time.Now()
	suggestCacheMu.Unlock()

	return items, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
