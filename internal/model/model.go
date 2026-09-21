package model

import (
	"github.com/golang-jwt/jwt/v4"
)

type PostURL struct {
	URL string `json:"URL"`
}

type ResultShortenURL struct {
	Result string `json:"result"`
}

type URLRecord struct {
	ShortURL string `json:"short_URL"`
	URLData  RequestURLData
}

type RequestURLData struct {
	OriginalURL   string `json:"original_URL"`
	CorrelationID string `json:"correlation_id"`
	UserID        uint64 `json:"user_id"`
	IsDeleted     bool   `json:"is_deleted"`
}

type ResponcePostURLData struct {
	ShortURL      string `json:"short_URL"`
	CorrelationID string `json:"correlation_id"`
}

type ResponceUserURLData struct {
	ShortURL    string `json:"short_URL"`
	OriginalURL string `json:"original_URL"`
}

type Claims struct {
	jwt.RegisteredClaims
	UserID uint64
}

type URLDataToSaveToFile struct {
	CurrentUserID uint64      `json:"current_user_id"`
	URLRecords    []URLRecord `json:"URL_records"`
}
