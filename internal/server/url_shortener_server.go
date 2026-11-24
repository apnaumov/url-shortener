// Package server запускает HTTP-сервер сервиса сокращения URL.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/repository"
)

// StartURLShortenerServer инициализирует конфигурацию, логгер и хранилище (PostgreSQL или in-memory),
// запускает сервер и блокируется до SIGINT/SIGTERM, после чего завершает работу корректно.
func StartURLShortenerServer() error {
	conf, err := config.InitConfig()
	if err != nil {
		return err
	}
	err = logger.SetLogDirectory(conf.LogDirectory)
	if err != nil {
		return err
	}

	logger, err := logger.InitializeRootLogger("server", "info")
	if err != nil {
		return err
	}
	zap.RedirectStdLog(logger)

	var storage repository.URLStorage
	logger.Info("Try to initialize storage")
	if len(conf.DBConnectionString) != 0 {
		st, err := repository.NewDBStorage(conf.DBConnectionString, &conf.PendingMessageProcessorParams, logger.Named("storage"))
		if err != nil {
			return err
		}
		logger.Info("Db storage initialized")
		storage = st
	} else {
		st, err := repository.NewRuntimeStorage(conf.FileStoragePath, &conf.PendingMessageProcessorParams, logger.Named("storage"))
		if err != nil {
			return err
		}
		logger.Info("Runtime storage initialized")
		storage = st
	}

	router, err := handler.NewURLShortenerRouter(conf.ServerBaseURL, conf.SecretKey, storage, conf.AuditServiceParams)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:    conf.ServerListenAddr,
		Handler: router.Mux,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("Starting server", zap.String("address", conf.ServerListenAddr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-sigChan:
		logger.Info("Received signal", zap.String("signal", sig.String()))
	}

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	router.OnShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shut down failed: %w", err)
	}

	logger.Info("Server gracefully shut down")

	return nil
}
