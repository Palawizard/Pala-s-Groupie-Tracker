package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"palasgroupietracker/internal/handlers"
	"palasgroupietracker/internal/store"
)

// main is the HTTP server entrypoint
func main() {
	// Load local env vars for API keys when running outside a managed environment
	err := godotenv.Load()
	if err != nil {
		// .env is optional in prod, so keep going if it's missing
		log.Println("could not load .env file:", err)
	}

	// Use an explicit mux so routes are easy to reason about
	mux := http.NewServeMux()
	ctx := context.Background()
	dbStore, err := store.OpenFromEnv(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNoDatabaseURL) {
			log.Println("database not configured; auth and favorites are disabled")
		} else {
			log.Fatal(err)
		}
	} else {
		defer dbStore.Close()
		handlers.SetStore(dbStore)
	}

	mux.HandleFunc("/artists", handlers.ArtistsHandler)
	mux.HandleFunc("/artists/ajax", handlers.ArtistsAjaxHandler)
	mux.HandleFunc("/artists/suggest", handlers.ArtistsSuggestHandler)
	mux.HandleFunc("/artists/", handlers.ArtistDetailHandler)
	mux.HandleFunc("/favorites", handlers.FavoritesHandler)
	mux.HandleFunc("/favorites/toggle", handlers.ToggleFavoriteHandler)
	mux.HandleFunc("/login", handlers.LoginHandler)
	mux.HandleFunc("/register", handlers.RegisterHandler)
	mux.HandleFunc("/logout", handlers.LogoutHandler)

	// Serve static assets from `web/static` under the `/static/` URL prefix
	fileServer := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Treat `/` as home, anything else as a 404 without a separate router
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { // route root and fallback to 404
		if r.URL.Path == "/" {
			handlers.HomeHandler(w, r)
			return
		}
		handlers.NotFound(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		// Local dev default. On Scalingo/Heroku-like platforms, PORT is injected.
		port = "8080"
	}
	addr := ":" + port
	log.Println("listening on", addr)
	// Start the HTTP server and fail hard if binding or serving fails
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		log.Fatal(err)
	}
}
