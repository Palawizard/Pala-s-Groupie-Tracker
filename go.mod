module palasgroupietracker

go 1.25.0

// +scalingo install ./cmd/server

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.11.1
	golang.org/x/oauth2 v0.36.0
)

require github.com/go-jose/go-jose/v4 v4.1.4 // indirect
