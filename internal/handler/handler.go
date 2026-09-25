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

type URLShortenerRouter struct {
	Mux           *chi.Mux
	service       *service.URLShortenerService
	jwtClient     *jwtClient
	requestLogger *zap.Logger
}

func NewURLShortenerRouter(URLBaseAddr string, authKey []byte, URLStorage repository.URLStorage) (*URLShortenerRouter, error) {
	URLShortenerRouter := &URLShortenerRouter{}
	URLShortenerRouter.Mux = chi.NewRouter()

	requestLogger, err := logger.InitializeRootLogger("server_requests", "info")

	if err != nil {
		return nil, err
	}

	URLShortenerRouter.requestLogger = requestLogger

	shortener, err := service.NewURLShortenerService(URLBaseAddr, URLStorage)

	if err != nil {
		return nil, err
	}

	URLShortenerRouter.service = shortener
	URLShortenerRouter.jwtClient = NewJWTClient(URLStorage, authKey)

	URLShortenerRouter.Mux.Use(URLShortenerRouter.getLoggerMiddleware)
	URLShortenerRouter.Mux.Use(URLShortenerRouter.getAuthMiddleware)
	URLShortenerRouter.Mux.Use(URLShortenerRouter.gzipMiddleware)
	URLShortenerRouter.Mux.Post("/", URLShortenerRouter.postNewURL)
	URLShortenerRouter.Mux.Get("/{shortPath}", URLShortenerRouter.getFullURL)
	URLShortenerRouter.Mux.Get("/ping", URLShortenerRouter.pingDB)
	URLShortenerRouter.Mux.MethodNotAllowed(URLShortenerRouter.methodNotAllowed)
	URLShortenerRouter.setAPIHandlers()

	return URLShortenerRouter, nil
}

func (router *URLShortenerRouter) OnShutdown() {
	if err := router.service.OnServerShutdown(); err != nil {
		router.requestLogger.Warn("Error while save service's configuration on shutdown", zap.String("error", err.Error()))
	} else {
		router.requestLogger.Info("Service's configuration saved on shutdown")
	}
}

func (router *URLShortenerRouter) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Method not allowed", http.StatusBadRequest)
}

func (router *URLShortenerRouter) postNewURL(w http.ResponseWriter, r *http.Request) {
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

	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	responseData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: string(body), UserID: userID})
	var status int

	if err != nil {
		if errors.Is(err, repository.ErrFullURLCollision) {
			router.requestLogger.Warn(repository.ErrFullURLCollision.Error(),
				zap.String("short_URL", responseData.ShortURL), zap.String("correlation_id", responseData.CorrelationID))
			status = http.StatusConflict
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		status = http.StatusCreated
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)

	w.Write([]byte(responseData.ShortURL))
}

func (router *URLShortenerRouter) getFullURL(w http.ResponseWriter, r *http.Request) {
	shortPath := chi.URLParam(r, "shortPath")
	URLData, err := router.service.GetFullURL(r.Context(), shortPath)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			router.requestLogger.Warn(err.Error())
			http.Error(w, "Invalid URL in request", http.StatusBadRequest)
		} else if errors.Is(err, repository.ErrDeleted) {
			router.requestLogger.Info(err.Error())
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
		} else {
			router.requestLogger.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Location", URLData)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (router *URLShortenerRouter) pingDB(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := router.service.GetStorage().Ping(ctx)

	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
