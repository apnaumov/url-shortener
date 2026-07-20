package router

import (
	"net/http"
	"strings"

	"github.com/apnaumov/url-shortener.git/internal/handler"
)

type UrlShortenerServer struct {
}

func (serv UrlShortenerServer) StartUrlShortenerServer() {
	mux := http.NewServeMux()
	addr := "localhost:8080"

	mux.Handle(`/`, handler.RootMiddleware(strings.Join([]string{"http://", addr}, "")))

	err := http.ListenAndServe(addr, mux)
	if err != nil {
		panic(err)
	}
}
