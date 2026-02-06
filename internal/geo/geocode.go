package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Geocoder resolves place names into coordinates using the Open-Meteo geocoding API.
// It keeps an in-memory cache to avoid repeated network calls.
type Geocoder struct {
	client *http.Client

	mu    sync.Mutex
	cache map[string]cachedResult
}

type cachedResult struct {
	result Result
	ok     bool
	at     time.Time
}

type Result struct {
	Lat     float64
	Lng     float64
	Display string
}

func NewGeocoder() *Geocoder {
	return &Geocoder{
		client: &http.Client{Timeout: 6 * time.Second},
		cache:  make(map[string]cachedResult),
	}
}

// Geocode returns coordinates for (name, countryCode). countryCode should be ISO-3166-1 alpha-2 (e.g. "FR", "US").
// If countryCode is provided, results outside that country are rejected (rather than picking a wrong match).
func (g *Geocoder) Geocode(ctx context.Context, name string, countryCode string) (Result, bool, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return Result{}, false, nil
	}

	cc := strings.ToUpper(strings.TrimSpace(countryCode))
	key := strings.ToLower(n) + "|" + cc

	g.mu.Lock()
	if hit, ok := g.cache[key]; ok {
		g.mu.Unlock()
		return hit.result, hit.ok, nil
	}
	g.mu.Unlock()

	res, ok, err := g.tryGeocode(ctx, n, cc)

	g.mu.Lock()
	g.cache[key] = cachedResult{result: res, ok: ok, at: time.Now()}
	g.mu.Unlock()

	return res, ok, err
}

func (g *Geocoder) tryGeocode(ctx context.Context, name string, countryCode string) (Result, bool, error) {
	// US states (and similar regions) are poorly handled by some city-focused geocoders.
	// If we recognize a state (even with a small typo), try Nominatim first.
	if countryCode == "US" {
		if norm, ok := normalizeUSStateName(name); ok {
			if res, ok2, err := g.geocodeNominatim(ctx, norm, countryCode); err == nil && ok2 {
				return res, true, nil
			}
		}
	}

	// Try the raw name first.
	if res, ok, err := g.tryProviders(ctx, name, countryCode); err == nil && ok {
		return res, ok, nil
	}

	// If we have a country context, try light normalization to avoid silly mismatches
	// (e.g. "Arizone" -> "Arizona") while still rejecting out-of-country results.
	if countryCode == "US" {
		if norm, ok := normalizeUSStateName(name); ok && !strings.EqualFold(norm, name) {
			if res, ok, err := g.tryProviders(ctx, norm, countryCode); err == nil && ok {
				return res, ok, nil
			}
		}
	}

	return Result{}, false, nil
}

func (g *Geocoder) tryProviders(ctx context.Context, name string, countryCode string) (Result, bool, error) {
	res, ok, err := g.geocodeOpenMeteo(ctx, name, countryCode)
	if err == nil && ok {
		return res, true, nil
	}

	// Fallback: Nominatim can be more tolerant, but we still filter by countryCode when possible.
	res2, ok2, err2 := g.geocodeNominatim(ctx, name, countryCode)
	if err2 == nil && ok2 {
		return res2, true, nil
	}

	// Preserve a hard error if Open-Meteo failed (network / HTTP error).
	if err != nil {
		return Result{}, false, err
	}
	return Result{}, false, nil
}

func (g *Geocoder) geocodeOpenMeteo(ctx context.Context, name string, countryCode string) (Result, bool, error) {
	u, err := url.Parse("https://geocoding-api.open-meteo.com/v1/search")
	if err != nil {
		return Result{}, false, err
	}
	q := u.Query()
	q.Set("name", name)
	q.Set("count", "5")
	q.Set("language", "en")
	q.Set("format", "json")
	if strings.TrimSpace(countryCode) != "" {
		q.Set("country", strings.ToUpper(strings.TrimSpace(countryCode)))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0")

	resp, err := g.client.Do(req)
	if err != nil {
		return Result{}, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{}, false, fmt.Errorf("open-meteo geocoding request failed: %s", resp.Status)
	}

	var body struct {
		Results []struct {
			Name        string  `json:"name"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
			Country     string  `json:"country"`
			CountryCode string  `json:"country_code"`
			Admin1      string  `json:"admin1"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, false, err
	}

	if len(body.Results) == 0 {
		return Result{}, false, nil
	}

	cc := strings.ToUpper(strings.TrimSpace(countryCode))
	candidates := body.Results
	if cc != "" {
		filtered := candidates[:0]
		for _, r := range candidates {
			if strings.ToUpper(strings.TrimSpace(r.CountryCode)) == cc {
				filtered = append(filtered, r)
			}
		}
		candidates = filtered
		if len(candidates) == 0 {
			return Result{}, false, nil
		}
	}

	best := candidates[0]
	bestScore := scoreCandidate(name, best.Name, best.Admin1)
	for i := 1; i < len(candidates); i++ {
		s := scoreCandidate(name, candidates[i].Name, candidates[i].Admin1)
		if s > bestScore {
			best = candidates[i]
			bestScore = s
		}
	}

	display := strings.TrimSpace(best.Name)
	if strings.TrimSpace(best.Admin1) != "" && !strings.EqualFold(best.Admin1, best.Name) {
		display = display + ", " + strings.TrimSpace(best.Admin1)
	}
	if strings.TrimSpace(best.Country) != "" {
		display = display + ", " + strings.TrimSpace(best.Country)
	}

	return Result{Lat: best.Latitude, Lng: best.Longitude, Display: display}, true, nil
}

func (g *Geocoder) geocodeNominatim(ctx context.Context, name string, countryCode string) (Result, bool, error) {
	u, err := url.Parse("https://nominatim.openstreetmap.org/search")
	if err != nil {
		return Result{}, false, err
	}
	q := u.Query()
	q.Set("q", name)
	q.Set("format", "jsonv2")
	q.Set("limit", "5")
	q.Set("addressdetails", "1")
	cc := strings.ToLower(strings.TrimSpace(countryCode))
	if cc != "" {
		q.Set("countrycodes", cc)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GroupieTrackerSchoolProject/1.0")

	resp, err := g.client.Do(req)
	if err != nil {
		return Result{}, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{}, false, fmt.Errorf("nominatim geocoding request failed: %s", resp.Status)
	}

	var body []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
		Address     struct {
			CountryCode string `json:"country_code"`
			State       string `json:"state"`
			Country     string `json:"country"`
		} `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, false, err
	}
	if len(body) == 0 {
		return Result{}, false, nil
	}

	candidates := body
	if cc != "" {
		filtered := candidates[:0]
		for _, r := range candidates {
			if strings.ToLower(strings.TrimSpace(r.Address.CountryCode)) == cc {
				filtered = append(filtered, r)
			}
		}
		candidates = filtered
		if len(candidates) == 0 {
			return Result{}, false, nil
		}
	}

	best := candidates[0]
	bestName := strings.TrimSpace(best.Name)
	if bestName == "" {
		bestName = strings.TrimSpace(best.DisplayName)
	}
	bestScore := scoreCandidate(name, bestName, best.Address.State)
	for i := 1; i < len(candidates); i++ {
		cn := strings.TrimSpace(candidates[i].Name)
		if cn == "" {
			cn = strings.TrimSpace(candidates[i].DisplayName)
		}
		s := scoreCandidate(name, cn, candidates[i].Address.State)
		if s > bestScore {
			best = candidates[i]
			bestScore = s
			bestName = cn
		}
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(best.Lat), 64)
	if err != nil {
		return Result{}, false, err
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(best.Lon), 64)
	if err != nil {
		return Result{}, false, err
	}

	display := strings.TrimSpace(best.DisplayName)
	if display == "" {
		display = strings.TrimSpace(bestName)
	}

	return Result{Lat: lat, Lng: lng, Display: display}, true, nil
}

// HumanizeLocationKey converts Groupie location keys like "new_south_wales-australia" into "New South Wales, Australia".
func HumanizeLocationKey(key string) string {
	place, country := splitLocationKey(key)
	place = titleWords(place)
	country = titleWords(country)
	if country == "Usa" {
		country = "USA"
	}
	if country == "Uk" {
		country = "UK"
	}
	if place == "" {
		return country
	}
	if country == "" {
		return place
	}
	return place + ", " + country
}

// QueryFromLocationKey returns a (placeName, countryCode, displayName) triple suitable for geocoding and UI.
func QueryFromLocationKey(key string) (string, string, string) {
	place, _ := splitLocationKey(key)
	place = strings.TrimSpace(place)
	return titleWords(place), CountryCodeFromKey(key), HumanizeLocationKey(key)
}

// CountryCodeFromKey tries to map the "-country" suffix to an ISO-3166-1 alpha-2 code for better disambiguation.
func CountryCodeFromKey(key string) string {
	_, country := splitLocationKey(key)
	c := strings.ToLower(strings.TrimSpace(country))
	c = strings.ReplaceAll(c, "_", " ")
	switch c {
	case "usa", "united states", "united states of america":
		return "US"
	case "uk", "united kingdom":
		return "GB"
	case "france":
		return "FR"
	case "switzerland":
		return "CH"
	case "australia":
		return "AU"
	case "new zealand":
		return "NZ"
	case "japan":
		return "JP"
	case "indonesia":
		return "ID"
	case "hungary":
		return "HU"
	case "belarus":
		return "BY"
	case "slovakia":
		return "SK"
	case "mexico":
		return "MX"
	case "french polynesia":
		return "PF"
	case "new caledonia":
		return "NC"
	default:
		return ""
	}
}

func splitLocationKey(key string) (string, string) {
	k := strings.TrimSpace(key)
	if k == "" {
		return "", ""
	}

	parts := strings.Split(k, "-")
	if len(parts) == 1 {
		return strings.ReplaceAll(parts[0], "_", " "), ""
	}

	country := parts[len(parts)-1]
	place := strings.Join(parts[:len(parts)-1], "-")
	place = strings.ReplaceAll(place, "_", " ")
	place = strings.ReplaceAll(place, "-", " ")
	country = strings.ReplaceAll(country, "_", " ")
	return place, country
}

func titleWords(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	for i := range words {
		w := words[i]
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

func scoreCandidate(query, name, admin1 string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	n := strings.ToLower(strings.TrimSpace(name))
	a := strings.ToLower(strings.TrimSpace(admin1))

	score := 0
	if n == q {
		score += 100
	}
	if strings.HasPrefix(n, q) && q != "" {
		score += 40
	}
	if strings.Contains(n, q) && q != "" {
		score += 20
	}
	// Encourage results that match even with small typos.
	if q != "" && n != "" {
		d := levenshtein(q, n)
		if d == 0 {
			score += 30
		} else if d == 1 {
			score += 20
		} else if d == 2 {
			score += 10
		}
	}
	if a != "" && q != "" && strings.Contains(a, q) {
		score += 5
	}
	return score
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// DP with two rows.
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		cur[0] = i
		ai := a[i-1]
		for j := 1; j <= len(b); j++ {
			cost := 0
			if ai != b[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min3(del, ins, sub)
		}
		prev, cur = cur, prev
	}

	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}

func normalizeUSStateName(s string) (string, bool) {
	q := strings.ToLower(strings.TrimSpace(s))
	q = strings.ReplaceAll(q, ",", " ")
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return "", false
	}

	states := []string{
		"alabama", "alaska", "arizona", "arkansas", "california", "colorado", "connecticut",
		"delaware", "florida", "georgia", "hawaii", "idaho", "illinois", "indiana", "iowa",
		"kansas", "kentucky", "louisiana", "maine", "maryland", "massachusetts", "michigan",
		"minnesota", "mississippi", "missouri", "montana", "nebraska", "nevada",
		"new hampshire", "new jersey", "new mexico", "new york", "north carolina",
		"north dakota", "ohio", "oklahoma", "oregon", "pennsylvania", "rhode island",
		"south carolina", "south dakota", "tennessee", "texas", "utah", "vermont",
		"virginia", "washington", "west virginia", "wisconsin", "wyoming",
	}

	best := ""
	bestD := 999
	for _, st := range states {
		d := levenshtein(q, st)
		if d < bestD {
			bestD = d
			best = st
		}
	}

	// Only accept exact match or small typos to avoid turning unrelated cities into states.
	if best != "" && bestD <= 2 {
		return titleWords(best), true
	}
	return "", false
}
