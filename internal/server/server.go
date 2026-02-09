package server

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

// Run bootstraps the app and blocks serving HTTP. It logs fatal on unrecoverable errors
// Keeping this in internal/server allows cmd/server/main.go to stay minimal.
func Run() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// Load local env vars for API keys when running outside a managed environment
	// In managed platforms (Scalingo), env vars are injected and there's no `.env` file
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Println("could not load .env file:", err)
		}
	}

	mux := http.NewServeMux()

	dbStore, err := store.OpenFromEnv(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNoDatabaseURL) {
			log.Println("database not configured; auth and favorites are disabled")
		} else {
			return err
		}
	} else {
		defer dbStore.Close()
		handlers.SetStore(dbStore)
	}

	registerRoutes(mux)

	port := os.Getenv("PORT")
	if port == "" {
		// Local dev default. On Scalingo/Heroku-like platforms, PORT is injected
		port = "8080"
	}
	addr := ":" + port
	log.Println("listening on", addr)

	return http.ListenAndServe(addr, mux)
}

func registerRoutes(mux *http.ServeMux) {
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			handlers.HomeHandler(w, r)
			return
		}
		handlers.NotFound(w, r)
	})
}
