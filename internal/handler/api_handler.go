package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/go-chi/chi/v5"
)

func (router *UrlShortenerRouter) setApiHandlers() {
	router.Mux.Route("/api", func(r chi.Router) {
		r.Post("/shorten", router.apiPostUrl)
	})
}

func (router *UrlShortenerRouter) apiPostUrl(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-type incorrect", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var postURL model.PostURL

	if err := json.NewDecoder(r.Body).Decode(&postURL); err != nil {
		router.requestLogger.Error(err.Error())
		if errors.Is(err, io.EOF) {
			http.Error(w, "Body must be not empty", http.StatusBadRequest)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	if len(postURL.URL) == 0 {
		http.Error(w, "URL must be not empty", http.StatusBadRequest)
		return
	}

	fullURL, err := router.service.SetFullURL(r.Context(), postURL.URL)
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(model.ResultShortenURL{Result: fullURL}); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}
