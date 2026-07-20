package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/apnaumov/url-shortener.git/internal/service"
)

var shortenerService = service.NewUrlShortenerService()

func RootMiddleware(fullAddr string) http.Handler {
	log.SetOutput(os.Stdout)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodPost {
			err := postNewURL(w, r, fullAddr)
			if err != nil {
				log.Print(err.Error())
				http.Error(w, err.Error(), 400)
			}
			return
		}

		if r.Method == http.MethodGet {
			err := getFullURL(w, r)
			if err != nil {
				log.Print(err.Error())
				http.Error(w, err.Error(), 400)
			}
			return
		}
		log.Print("Method not allowed")
		http.Error(w, "Method not allowed", 400)
	})
}

func postNewURL(w http.ResponseWriter, r *http.Request, serverAddr string) error {
	if r.URL.Path != "/" {
		return fmt.Errorf("Method not allowed")
	}

	if r.Header.Get("Content-Type") != "text/plain" {
		return fmt.Errorf("Content-type incorrect")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if len(body) == 0 {
		return fmt.Errorf("Body must be not empty")
	}

	fullURL := serverAddr + "/" + shortenerService.SetFullURL(string(body))

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(fullURL))

	return nil
}

func getFullURL(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Path == "/" {
		return fmt.Errorf("Method not allowed")
	}

	shortPath := strings.TrimPrefix(r.URL.Path, "/")
	fullURL, err := shortenerService.GetFullURL(shortPath)
	if err != nil {
		return err
	}

	w.Header().Set("Location", fullURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	return nil
}
