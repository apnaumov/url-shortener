package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
)

type UrlShortenerRouter struct {
	Mux     *chi.Mux
	service *service.UrlShortenerService
}

func NewUrlShortenerRouter(urlBaseAddr string) (*UrlShortenerRouter, error) {
	urlShortenerRouter := &UrlShortenerRouter{}
	urlShortenerRouter.Mux = chi.NewRouter()

	urlShortenerRouter.Mux.Post("/", urlShortenerRouter.postNewURL)
	urlShortenerRouter.Mux.Get("/{shortPath}", urlShortenerRouter.getFullURL)
	urlShortenerRouter.Mux.MethodNotAllowed(urlShortenerRouter.methodNotAllowed)

	shortener, err := service.NewUrlShortenerService(urlBaseAddr)

	if err != nil {
		return nil, err
	}

	urlShortenerRouter.service = shortener

	return urlShortenerRouter, nil
}

func (router *UrlShortenerRouter) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Method not allowed", http.StatusBadRequest)
}

func (router *UrlShortenerRouter) postNewURL(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-type incorrect", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Body must be not empty", http.StatusBadRequest)
		return
	}

	fullURL, err := router.service.SetFullURL(string(body))
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(fullURL))
}

func (router *UrlShortenerRouter) getFullURL(w http.ResponseWriter, r *http.Request) {
	shortPath := chi.URLParam(r, "shortPath")
	fullURL, err := router.service.GetFullURL(shortPath)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Invalid URL in request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", fullURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
