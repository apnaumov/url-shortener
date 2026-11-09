package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/config"
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
	auditService  *service.AuditService
}

func NewURLShortenerRouter(URLBaseAddr string, authKey []byte, URLStorage repository.URLStorage, auditConf config.AuditServiceConfig) (*URLShortenerRouter, error) {
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

	audit, err := service.NewAuditService(auditConf)
	if err != nil {
		URLShortenerRouter.requestLogger.Warn("Can't initialize audit service", zap.Error(err))
	}

	URLShortenerRouter.service = shortener
	URLShortenerRouter.jwtClient = NewJWTClient(URLStorage, authKey)
	URLShortenerRouter.auditService = audit

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

	originalURL := string(body)

	responseData, err := router.service.SetFullURL(ctx, model.RequestURLData{OriginalURL: originalURL, UserID: userID})
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

	if status == http.StatusCreated && router.auditService != nil {
		errs := router.auditService.NotifyShorten(userID, originalURL)
		for i := range errs {
			router.requestLogger.Error("Audit service error.", zap.String("Error", errs[i].Error()))
		}
	}
}

func (router *URLShortenerRouter) getFullURL(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCtx(r.Context())
	if err != nil {
		router.requestLogger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

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

	if router.auditService != nil {
		errs := router.auditService.NotifyFollow(userID, URLData)
		for i := range errs {
			router.requestLogger.Error("Audit service error.", zap.String("Error", errs[i].Error()))
		}
	}
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
