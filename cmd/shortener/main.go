package main

import "github.com/apnaumov/url-shortener.git/internal/server"

func main() {
	server := server.UrlShortenerServer{}
	server.StartUrlShortenerServer()
}
