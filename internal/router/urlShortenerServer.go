package router

import (
	"net/http"

	"github.com/apnaumov/url-shortener.git/internal/handler"
)

type UrlShortenerServer struct {
}

func (serv UrlShortenerServer) StartUrlShortenerServer() {
	mux := http.NewServeMux()
	mux.Handle(`/`, handler.RootMiddleware())

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
