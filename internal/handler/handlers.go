package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apnaumov/url-shortener.git/internal/service"
)

var shortenerService = service.NewUrlShortenerService()

func RootMiddleware() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodPost {
			err := postNewURL(w, r)
			if err != nil {
				http.Error(w, err.Error(), 400)
			}
			return
		}

		if r.Method == http.MethodGet {
			err := getFullURL(w, r)
			if err != nil {
				http.Error(w, err.Error(), 400)
			}
			return
		}

		http.Error(w, "Method not allowed", 400)
	})
}

func postNewURL(w http.ResponseWriter, r *http.Request) error {
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

	shortUrl := shortenerService.SetFullURL(string(body))

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(shortUrl))

	return nil
}

func getFullURL(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Path == "/" {
		return fmt.Errorf("Method not allowed")
	}

	if r.Header.Get("Content-Type") != "text/plain" {
		return fmt.Errorf("Content-type incorrect")
	}

	shortPath := strings.TrimPrefix(r.URL.Path, "/")
	fullURL := shortenerService.GetFullURL(shortPath)

	w.Header().Set("Location", fullURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	return nil
}
