package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/go-chi/chi/v5"
)

func (router *UrlShortenerRouter) setApiHandlers() {
	router.Mux.Route("/api", func(r chi.Router) {
		r.Route("/shorten", func(r chi.Router) {
			r.Post("/", router.apiPostUrl)
			r.Post("/batch", router.apiPostUrlBatch)
		})
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responceData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: postURL.URL})
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(model.ResultShortenURL{Result: responceData.ShortUrl}); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (router *UrlShortenerRouter) apiPostUrlBatch(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-type incorrect", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var requestDataBatch []model.RequestURLData

	if err := json.NewDecoder(r.Body).Decode(&requestDataBatch); err != nil {
		router.requestLogger.Error(err.Error())
		if errors.Is(err, io.EOF) {
			http.Error(w, "Body must be not empty", http.StatusBadRequest)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responceDataBatch := make([]model.ResponceURLData, 0, len(requestDataBatch))

	for i := range requestDataBatch {
		responceData, err := router.service.SetFullURL(ctx, requestDataBatch[i])

		if err != nil {
			var status int
			if errors.Is(err, repository.CorrelationError) {
				router.requestLogger.Warn(err.Error())
				status = http.StatusConflict
			} else {
				router.requestLogger.Error(err.Error())
				status = http.StatusInternalServerError
			}

			http.Error(w, http.StatusText(status), status)
			return
		}

		responceDataBatch = append(responceDataBatch, responceData)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(responceDataBatch); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
