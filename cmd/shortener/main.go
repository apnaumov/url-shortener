package main

import (
	_ "net/http/pprof"

	"github.com/apnaumov/url-shortener.git/internal/server"
)

func main() {
	server.StartURLShortenerServer()
}
