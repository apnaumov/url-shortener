package handler

import (
	"io"
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
)

var ServerAddr string
var shortenerService = service.NewUrlShortenerService()

func InitRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Post("/", postNewURL)
	r.Get("/{shortPath}", getFullURL)
	r.MethodNotAllowed(methodNotAllowed)
	return r
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

	fullURL := ServerAddr + "/" + shortenerService.SetFullURL(string(body))

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
