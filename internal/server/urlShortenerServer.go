package server

import (
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
)

func StartUrlShortenerServer() {
	conf := config.InitConfigFromCLI()

	mux, err := handler.InitRouter(conf.ServerBaseUrl)
	if err != nil {
		panic(err)
	}

	err = http.ListenAndServe(conf.ServerListenAddr, mux)
	if err != nil {
		panic(err)
	}
}
