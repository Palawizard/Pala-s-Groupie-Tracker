package handlers

import (
	"net/http"
	"os"
	"strings"
)

// getBasePath returns a normalized base path (no trailing slash) used when the app
// is hosted under a subpath behind a reverse proxy (e.g. /groupie-tracker)
func getBasePath(r *http.Request) string {
	// Preferred: gateway sets this explicitly
	bp := strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix"))
	if bp == "" {
		// Fallback: allow configuring locally or on platforms without that header
		bp = strings.TrimSpace(os.Getenv("BASE_PATH"))
	}

	if bp == "" || bp == "/" {
		return ""
	}
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	bp = strings.TrimRight(bp, "/")
	if bp == "/" {
		return ""
	}
	return bp
}

func withBasePath(r *http.Request, p string) string {
	base := getBasePath(r)
	if base == "" {
		return p
	}
	if p == "" || p == "/" {
		return base + "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return base + p
}
