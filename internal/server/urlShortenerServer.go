package server

import (
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/handler"
)

type UrlShortenerServer struct {
}

func (serv UrlShortenerServer) StartUrlShortenerServer() {
	addr := "localhost:8080"
	handler.ServerAddr = "http://" + addr

	err := http.ListenAndServe(addr, handler.InitRouter())
	if err != nil {
		panic(err)
	}
}
