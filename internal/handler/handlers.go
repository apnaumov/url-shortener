package handler

import (
	"io"
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
)

var shortenerService *service.UrlShortenerService

func InitRouter(urlBaseAddr string) (*chi.Mux, error) {
	r := chi.NewRouter()
	r.Post("/", postNewURL)
	r.Get("/{shortPath}", getFullURL)
	r.MethodNotAllowed(methodNotAllowed)

	shortener, err := service.NewUrlShortenerService(urlBaseAddr)

	if err != nil {
		return nil, err
	}

	shortenerService = shortener

	return r, nil
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Method not allowed", http.StatusBadRequest)
}

func postNewURL(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-type incorrect", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Body must be not empty", http.StatusBadRequest)
		return
	}

	fullURL := shortenerService.SetFullURL(string(body))

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(fullURL))
}

func getFullURL(w http.ResponseWriter, r *http.Request) {
	shortPath := chi.URLParam(r, "shortPath")
	fullURL, err := shortenerService.GetFullURL(shortPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", fullURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
