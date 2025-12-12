package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"palasgroupietracker/internal/handlers"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("could not load .env file:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/artists", handlers.ArtistsHandler)
	mux.HandleFunc("/artists/ajax", handlers.ArtistsAjaxHandler)
	mux.HandleFunc("/artists/", handlers.ArtistDetailHandler)

	fileServer := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			handlers.HomeHandler(w, r)
			return
		}
		handlers.NotFound(w, r)
	})

	log.Println("listening on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
