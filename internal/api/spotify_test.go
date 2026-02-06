package api

import "testing"

func TestParseSpotifyReleaseDate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		ok   bool
		want string // YYYY-MM-DD
	}{
		{"", false, ""},
		{"   ", false, ""},
		{"2024-06-01", true, "2024-06-01"},
		{"2024-06", true, "2024-06-01"},
		{"2024", true, "2024-01-01"},
		{"2024-06-01T12:34:56Z", true, "2024-06-01"},
		{"2024-06-01T12:34:56.123Z", true, "2024-06-01"},
	}

	for _, tc := range cases {
		got, ok := ParseSpotifyReleaseDate(tc.in)
		if ok != tc.ok {
			t.Fatalf("ParseSpotifyReleaseDate(%q) ok=%v, want %v", tc.in, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("ParseSpotifyReleaseDate(%q)=%s, want %s", tc.in, got.Format("2006-01-02"), tc.want)
		}
	}
}

