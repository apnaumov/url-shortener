package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"

	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"go.uber.org/zap"
)

type URLShortenerService struct {
	shortenerURLs repository.URLStorage
	URLBaseAddr   string
	logger        *zap.Logger
}

func NewURLShortenerService(URLBaseAddr string, URLStorage repository.URLStorage) (*URLShortenerService, error) {
	URL, err := url.Parse(URLBaseAddr)
	if err != nil {
		return nil, err
	}

	shortenerLogger, err := logger.InitializeRootLogger("shortener_service", "info")

	if err != nil {
		return nil, err
	}

	URLShortenerService := &URLShortenerService{
		URLBaseAddr:   URL.String(),
		shortenerURLs: URLStorage,
		logger:        shortenerLogger,
	}

	return URLShortenerService, nil
}

func (shortenerService *URLShortenerService) GetStorage() repository.URLStorage {
	return shortenerService.shortenerURLs
}

func (shortenerService *URLShortenerService) OnServerShutdown() error {
	return shortenerService.shortenerURLs.OnServerShutdown()
}

func (shortenerService *URLShortenerService) GetFullURL(ctx context.Context, shortURL string) (string, error) {
	v, err := shortenerService.shortenerURLs.GetFullURL(ctx, shortURL)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", fmt.Errorf("can't find URL by the key %q. Error: %w", shortURL, err)
		}
		if errors.Is(err, repository.ErrDeleted) {
			return "", fmt.Errorf("Error: %w, shortUrl: %s", err, shortURL)
		}

		return "", err
	}
	return v, nil
}

func (shortenerService *URLShortenerService) GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error) {
	results, err := shortenerService.shortenerURLs.GetUserURLs(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range results {
		resShortURL, err := url.JoinPath(shortenerService.URLBaseAddr, results[i].ShortURL)
		if err != nil {
			return nil, err
		}
		results[i].ShortURL = resShortURL
	}

	return results, nil
}

func (shortenerService *URLShortenerService) DeleteUserURLs(ctx context.Context, userID uint64, shortURLs []string) error {
	return shortenerService.shortenerURLs.DeleteUserURLs(repository.DeleteUserURLsDTO{UserID: userID, ShortURLs: shortURLs})
}

func (shortenerService *URLShortenerService) SetFullURL(ctx context.Context, URLData model.RequestURLData) (model.ResponcePostURLData, error) {
	const maxAttemptsToGenerateKey = 10
	var responseData model.ResponcePostURLData
	var collisionErr error = nil
	for range maxAttemptsToGenerateKey {
		shortURL := generateShortKey()

		URLRecord := model.URLRecord{ShortURL: shortURL, URLData: URLData}

		resp, err := shortenerService.shortenerURLs.SetURL(ctx, URLRecord)

		if err != nil {
			if errors.Is(err, repository.ErrFullURLCollision) {
				collisionErr = repository.ErrFullURLCollision
			} else if errors.Is(err, repository.ErrShortURLCollision) {
				shortenerService.logger.Debug("Can't generate short key because of collision.", zap.String("short key", shortURL))
				continue
			} else {
				return model.ResponcePostURLData{}, err
			}
		}
		responseData = resp
		break
	}

	resShortURL, err := url.JoinPath(shortenerService.URLBaseAddr, responseData.ShortURL)
	if err != nil {
		return model.ResponcePostURLData{}, err
	}
	responseData.ShortURL = resShortURL

	return responseData, collisionErr
}

func (shortenerService *URLShortenerService) SetFullURLBatch(ctx context.Context, URLDatas []model.RequestURLData) ([]model.ResponcePostURLData, error) {
	const maxAttemptsToGenerateKey = 10
	responseDataBatch := make([]model.ResponcePostURLData, 0, len(URLDatas))
	var collisionErr error = nil

	attemptsToGenKeys := 0

	URLRecords := make([]model.URLRecord, 0, len(URLDatas))
	// generate URL records with empty short URL
	for i := range URLDatas {
		URLRecords = append(URLRecords, model.URLRecord{URLData: URLDatas[i]})
	}

	for range maxAttemptsToGenerateKey {
		attemptsToGenKeys++

		// set generated short URLs to URLRecords
		for i := range URLRecords {
			URLRecords[i].ShortURL = generateShortKey()
		}

		currentRespData, unacceptedURLRecords, err := shortenerService.shortenerURLs.SetURLBatch(ctx, URLRecords)
		responseDataBatch = append(responseDataBatch, currentRespData...)

		if err != nil {
			if errors.Is(err, repository.ErrFullURLCollision) {
				collisionErr = repository.ErrFullURLCollision
			} else {
				return nil, err
			}
		}

		if len(unacceptedURLRecords) != 0 {
			URLRecords = unacceptedURLRecords
		} else {
			break
		}
	}

	if attemptsToGenKeys == maxAttemptsToGenerateKey {
		return nil, repository.ErrShortURLCollision
	}

	for i := range responseDataBatch {
		resShortURL, err := url.JoinPath(shortenerService.URLBaseAddr, responseDataBatch[i].ShortURL)
		if err != nil {
			return nil, err
		}
		responseDataBatch[i].ShortURL = resShortURL
	}

	return responseDataBatch, collisionErr
}

// Generate a random short key (generated by AI)
func generateShortKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 6

	b := make([]byte, keyLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
