package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type UrlShortenerRouter struct {
	Mux           *chi.Mux
	service       *service.UrlShortenerService
	requestLogger *zap.Logger
}

func NewUrlShortenerRouter(urlBaseAddr string) (*UrlShortenerRouter, error) {
	urlShortenerRouter := &UrlShortenerRouter{}
	urlShortenerRouter.Mux = chi.NewRouter()

	requestLogger, err := logger.InitializeRootLogger("server_requests", "info")

	if err != nil {
		return nil, err
	}

	urlShortenerRouter.requestLogger = requestLogger

	shortener, err := service.NewUrlShortenerService(urlBaseAddr)

	if err != nil {
		return nil, err
	}

	urlShortenerRouter.service = shortener

	urlShortenerRouter.Mux.Use(urlShortenerRouter.getLoggerMiddleware)
	urlShortenerRouter.Mux.Post("/", urlShortenerRouter.postNewURL)
	urlShortenerRouter.Mux.Get("/{shortPath}", urlShortenerRouter.getFullURL)
	urlShortenerRouter.Mux.MethodNotAllowed(urlShortenerRouter.methodNotAllowed)

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
		router.requestLogger.Info(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Body must be not empty", http.StatusBadRequest)
		return
	}

	fullURL, err := router.service.SetFullURL(string(body))
	if err != nil {
		router.requestLogger.Info(err.Error())
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
		router.requestLogger.Info(err.Error())
		http.Error(w, "Invalid URL in request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", fullURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

type (
	// берём структуру для хранения сведений об ответе
	responseData struct {
		status int
		size   int
	}

	// добавляем реализацию http.ResponseWriter
	loggingResponseWriter struct {
		http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
		responseData        *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size // захватываем размер
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

func (router *UrlShortenerRouter) getLoggerMiddleware(h http.Handler) http.Handler {
	httpLogger := router.requestLogger.Named("http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var responseData responseData
		lw := &loggingResponseWriter{
			ResponseWriter: w,
			responseData:   &responseData,
		}
		h.ServeHTTP(lw, r)

		duration := time.Since(start)

		httpLogger.Sugar().Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"request's process time", duration,
			"response's status", responseData.status,
			"response's size", responseData.size,
		)
	})
}
