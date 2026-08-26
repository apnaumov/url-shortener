package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/apnaumov/url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type UrlShortenerRouter struct {
	Mux           *chi.Mux
	service       *service.UrlShortenerService
	requestLogger *zap.Logger
}

func NewUrlShortenerRouter(urlBaseAddr string, urlStorage repository.UrlStorage) (*UrlShortenerRouter, error) {
	urlShortenerRouter := &UrlShortenerRouter{}
	urlShortenerRouter.Mux = chi.NewRouter()

	requestLogger, err := logger.InitializeRootLogger("server_requests", "info")

	if err != nil {
		return nil, err
	}

	urlShortenerRouter.requestLogger = requestLogger

	shortener, err := service.NewUrlShortenerService(urlBaseAddr, urlStorage)

	if err != nil {
		return nil, err
	}

	urlShortenerRouter.service = shortener

	urlShortenerRouter.Mux.Use(urlShortenerRouter.getLoggerMiddleware)
	urlShortenerRouter.Mux.Use(urlShortenerRouter.gzipMiddleware)
	urlShortenerRouter.Mux.Post("/", urlShortenerRouter.postNewURL)
	urlShortenerRouter.Mux.Get("/{shortPath}", urlShortenerRouter.getFullURL)
	urlShortenerRouter.Mux.Get("/ping", urlShortenerRouter.pingDb)
	urlShortenerRouter.Mux.MethodNotAllowed(urlShortenerRouter.methodNotAllowed)
	urlShortenerRouter.setApiHandlers()

	return urlShortenerRouter, nil
}

func (router *UrlShortenerRouter) OnShutdown() {
	if err := router.service.OnServerShutdown(); err != nil {
		router.requestLogger.Warn("Error while save service's configuration on shutdown", zap.String("error", err.Error()))
	} else {
		router.requestLogger.Info("Service's configuration saved on shutdown")
	}
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
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Body must be not empty", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	responceData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: string(body)})

	if err != nil {
		var targetErr *repository.FullUrlCollisionError
		if errors.As(err, &targetErr) {
			router.requestLogger.Warn(targetErr.Error())
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)

			w.Write([]byte(targetErr.ShortUrl))
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(responceData.ShortUrl))
}

func (router *UrlShortenerRouter) getFullURL(w http.ResponseWriter, r *http.Request) {
	shortPath := chi.URLParam(r, "shortPath")
	urlData, err := router.service.GetFullURL(r.Context(), shortPath)
	if err != nil {
		if errors.Is(err, repository.NotFoundError) {
			router.requestLogger.Warn(err.Error())
			http.Error(w, "Invalid URL in request", http.StatusBadRequest)
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Location", urlData.OriginalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (router *UrlShortenerRouter) pingDb(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var err error
	if storage, isDbType := router.service.GetStorage().(*repository.DbStorage); isDbType {
		err = storage.GetNativeDb().PingContext(ctx)
	} else {
		err = errors.New("storage not an DbStorage type")
	}

	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
