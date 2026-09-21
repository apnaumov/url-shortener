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

func (router *URLShortenerRouter) setAPIHandlers() {
	router.Mux.Route("/api", func(r chi.Router) {
		r.Route("/shorten", func(r chi.Router) {
			r.Post("/", router.apiPostURL)
			r.Post("/batch", router.apiPostURLBatch)
		})

		r.Route("/user/urls", func(r chi.Router) {
			r.Get("/", router.apiGetUserURLs)
			r.Delete("/", router.apiDelUserURLs)
		})
	})
}

func (router *URLShortenerRouter) apiGetUserURLs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	responseData, err := router.service.GetUserURLs(ctx, userID)
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(responseData) == 0 {
		http.Error(w, http.StatusText(http.StatusNoContent), http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (router *URLShortenerRouter) apiDelUserURLs(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	requestShortURLs := make([]string, 0)

	if !router.checkJSONRequest(w, r, &requestShortURLs) {
		return
	}

	if len(requestShortURLs) == 0 {
		http.Error(w, "URLs must be not empty", http.StatusBadRequest)
		return
	}

	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = router.service.DeleteUserURLs(ctx, userID, requestShortURLs)

	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if errors.Is(err, repository.ErrDeleteProhibited) {
		router.requestLogger.Warn("Delete urls by this user is prohibited", zap.Strings("URLs", requestShortURLs), zap.Uint64("User id", userID))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if errors.Is(err, repository.ErrDeleted) {
		router.requestLogger.Warn("Some Urls already deleted", zap.Strings("URLs", requestShortURLs), zap.Uint64("User id", userID))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if errors.Is(err, repository.ErrNotFound) {
		router.requestLogger.Warn("Some Urls not found", zap.Strings("URLs", requestShortURLs), zap.Uint64("User id", userID))
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	router.requestLogger.Error(err.Error())
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (router *URLShortenerRouter) apiPostURL(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var postURL model.PostURL

	if !router.checkJSONRequest(w, r, &postURL) {
		return
	}

	if len(postURL.URL) == 0 {
		http.Error(w, "URL must be not empty", http.StatusBadRequest)
		return
	}

	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responseData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: postURL.URL, UserID: userID})

	var status int
	var res model.ResultShortenURL

	if err != nil {
		if errors.Is(err, repository.ErrFullURLCollision) {
			router.requestLogger.Warn(repository.ErrFullURLCollision.Error(),
				zap.String("short_URL", responseData.ShortURL), zap.String("correlation_id", responseData.CorrelationID))
			status = http.StatusConflict
			res = model.ResultShortenURL{Result: responseData.ShortURL}
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		status = http.StatusCreated
		res = model.ResultShortenURL{Result: responseData.ShortURL}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(res); err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (router *URLShortenerRouter) apiPostURLBatch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var requestDataBatch []model.RequestURLData

	if !router.checkJSONRequest(w, r, &requestDataBatch) {
		return
	}

	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for i := range requestDataBatch {
		if len(requestDataBatch[i].CorrelationID) == 0 || len(requestDataBatch[i].OriginalURL) == 0 {
			http.Error(w, "URL data must be not empty", http.StatusBadRequest)
			return
		}

		requestDataBatch[i].UserID = userID
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responseDataBatch, err := router.service.SetFullURLBatch(ctx, requestDataBatch)

	var status int
	if err != nil {
		if errors.Is(err, repository.ErrFullURLCollision) {
			router.requestLogger.Warn(repository.ErrFullURLCollision.Error(), zap.Int("batch size", len(responseDataBatch)))
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

func (router *URLShortenerRouter) checkJSONRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-type incorrect", http.StatusBadRequest)
		return false
	}

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		router.requestLogger.Error(err.Error())
		if errors.Is(err, io.EOF) {
			http.Error(w, "Body must be not empty", http.StatusBadRequest)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return false
	}

	return true
}
