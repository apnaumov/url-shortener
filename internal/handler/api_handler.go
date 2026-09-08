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
	"go.uber.org/zap"
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

	responseData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: postURL.URL})

	var status int
	var res model.ResultShortenURL

	if err != nil {
		if errors.Is(err, repository.FullUrlCollisionError) {
			router.requestLogger.Warn(repository.FullUrlCollisionError.Error(),
				zap.String("short_url", responseData.ShortUrl), zap.String("correlation_id", responseData.CorrelationId))
			status = http.StatusConflict
			res = model.ResultShortenURL{Result: responseData.ShortUrl}
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		status = http.StatusCreated
		res = model.ResultShortenURL{Result: responseData.ShortUrl}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(res); err != nil {
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

	for i := range requestDataBatch {
		if len(requestDataBatch[i].CorrelationId) == 0 || len(requestDataBatch[i].OriginalURL) == 0 {
			http.Error(w, "URL data must be not empty", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responseDataBatch, err := router.service.SetFullURLBatch(ctx, requestDataBatch)

	var status int
	if err != nil {
		if errors.Is(err, repository.FullUrlCollisionError) {
			router.requestLogger.Warn(repository.FullUrlCollisionError.Error(), zap.Int("batch size", len(responseDataBatch)))
			status = http.StatusConflict
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		status = http.StatusCreated
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(responseDataBatch); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
