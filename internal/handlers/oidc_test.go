package handlers

import (
	"net/url"
	"testing"
)

func TestEnrollmentURL(t *testing.T) {
	authz := "https://auth.example.org/application/o/authorize/?client_id=abc&state=xyz"

	t.Setenv("OIDC_ENROLLMENT_URL", "")
	if got := enrollmentURL(authz); got != authz {
		t.Fatalf("without enrollment URL: got %q", got)
	}

	t.Setenv("OIDC_ENROLLMENT_URL", "https://auth.example.org/if/flow/inscription/")
	got, err := url.Parse(enrollmentURL(authz))
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "auth.example.org" || got.Path != "/if/flow/inscription/" {
		t.Fatalf("unexpected sign-up URL %q", got)
	}
	if next := got.Query().Get("next"); next != "/application/o/authorize/?client_id=abc&state=xyz" {
		t.Fatalf("next must be the relative authorization URL, got %q", next)
	}

	// Authentik only follows a relative next: another host falls back to plain sign-in.
	t.Setenv("OIDC_ENROLLMENT_URL", "https://other.example.org/if/flow/inscription/")
	if got := enrollmentURL(authz); got != authz {
		t.Fatalf("other host: got %q", got)
	}
}
