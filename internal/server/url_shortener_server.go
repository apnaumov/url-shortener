package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
	"github.com/apnaumov/url-shortener.git/internal/logger"
	"go.uber.org/zap"
)

func StartUrlShortenerServer() {
	conf := config.InitConfig()
	err := logger.SetLogDirectory(conf.LogDirectory)
	if err != nil {
		log.Fatal(err.Error())
	}

	logger, err := logger.InitializeRootLogger("server", "info")
	if err != nil {
		log.Fatal(err.Error())
	}
	zap.RedirectStdLog(logger)

	router, err := handler.NewUrlShortenerRouter(conf.ServerBaseUrl, conf.FileStoragePath)
	if err != nil {
		logger.Fatal(err.Error())
	}

	srv := &http.Server{
		Addr:    conf.ServerListenAddr,
		Handler: router.Mux,
	}

	go func() {
		logger.Info("Starting server", zap.String("address", conf.ServerListenAddr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(err.Error())
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan

	logger.Info("Received signal", zap.String("signal", sig.String()))

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	router.OnShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server shut down failed", zap.String("error", err.Error()))
	}

	logger.Info("Server gracefully shut down")
}
