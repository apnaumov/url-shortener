package model

import (
	"github.com/golang-jwt/jwt/v4"
)

type PostURL struct {
	URL string `json:"url"`
}

type ResultShortenURL struct {
	Result string `json:"result"`
}

type URLRecord struct {
	ShortURL string `json:"short_url"`
	UrlData  RequestURLData
}

type RequestURLData struct {
	OriginalURL   string `json:"original_url"`
	CorrelationId string `json:"correlation_id"`
	UserId        uint64 `json:"user_id"`
}

type ResponcePostURLData struct {
	ShortUrl      string `json:"short_url"`
	CorrelationId string `json:"correlation_id"`
}

type ResponceUserURLData struct {
	ShortUrl    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type Claims struct {
	jwt.RegisteredClaims
	UserID uint64
}

type UrlDataToSaveToFile struct {
	CurrentUserId uint64      `json:"current_user_id"`
	UrlRecords    []URLRecord `json:"url_records"`
}
