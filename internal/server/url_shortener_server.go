package server

import (
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
	"github.com/apnaumov/url-shortener.git/internal/logger"
)

func StartUrlShortenerServer() {
	logger, err := logger.InitializeRootLogger("server", "info")
	conf := config.InitConfig()

	router, err := handler.NewUrlShortenerRouter(conf.ServerBaseUrl)
	if err != nil {
		logger.Fatal(err.Error())
	}

	err = http.ListenAndServe(conf.ServerListenAddr, router.Mux)
	if err != nil {
		logger.Fatal(err.Error())
	}
}
