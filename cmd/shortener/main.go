// Command shortener запускает сервис сокращения URL.
package main

import (
	"log"
	_ "net/http/pprof"

	"github.com/apnaumov/url-shortener.git/internal/server"
)

func main() {
	if err := server.StartURLShortenerServer(); err != nil {
		log.Fatal(err)
	}
}
