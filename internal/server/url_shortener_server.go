package server

import (
	"log"
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
)

func StartUrlShortenerServer() {
	conf := config.InitConfigFromCLI()

	router, err := handler.NewUrlShortenerRouter(conf.ServerBaseUrl)
	if err != nil {
		log.Fatal(err)
	}

	err = http.ListenAndServe(conf.ServerListenAddr, router.Mux)
	if err != nil {
		log.Fatal(err)
	}
}
