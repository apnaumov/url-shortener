package service

import (
	"math/rand"
	"time"
)

type UrlShortenerService struct {
	shortenerUrls map[string]string
}

func NewUrlShortenerService() *UrlShortenerService {
	return &UrlShortenerService{
		shortenerUrls: make(map[string]string),
	}
}

func (shortenerService UrlShortenerService) GetFullURL(shortURL string) string {
	return shortenerService.shortenerUrls[shortURL]
}

func (shortenerService *UrlShortenerService) SetFullURL(fullURL string) string {
	// short URL
	shortURL := generateShortKey()

	shortenerService.shortenerUrls[shortURL] = fullURL

	return shortURL
}

// Generate a random short key
func generateShortKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 6

	rand.Seed(time.Now().UnixNano())
	b := make([]byte, keyLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
