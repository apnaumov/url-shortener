package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type UrlShortenerService struct {
	mu            sync.RWMutex
	shortenerUrls map[string]string
}

func NewUrlShortenerService() *UrlShortenerService {
	return &UrlShortenerService{
		shortenerUrls: make(map[string]string),
	}
}

func (shortenerService UrlShortenerService) GetFullURL(shortURL string) (string, error) {
	shortenerService.mu.RLock()
	defer shortenerService.mu.RUnlock()
	v, ok := shortenerService.shortenerUrls[shortURL]
	if !ok {
		return "", fmt.Errorf("Can't find URL")
	}
	return v, nil
}

func (shortenerService *UrlShortenerService) SetFullURL(fullURL string) string {

	// short URL
	shortURL := generateShortKey()

	shortenerService.mu.Lock()
	defer shortenerService.mu.Unlock()
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
