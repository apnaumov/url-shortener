package main

import "github.com/apnaumov/url-shortener.git/internal/router"

func main() {
	server := router.UrlShortenerServer{}
	server.StartUrlShortenerServer()
}
