// Package model содержит общие структуры данных сервиса.
package model

import (
	"github.com/golang-jwt/jwt/v4"
)

// PostURL — тело запроса POST /api/shorten.
type PostURL struct {
	URL string `json:"URL"`
}

// ResultShortenURL — ответ POST /api/shorten с сокращённым URL.
type ResultShortenURL struct {
	Result string `json:"result"`
}

// URLRecord — запись хранилища: короткий ключ и данные исходного URL.
type URLRecord struct {
	ShortURL string `json:"short_URL"`
	URLData  RequestURLData
}

// RequestURLData — данные URL, пришедшие в запросе на сокращение.
type RequestURLData struct {
	OriginalURL   string `json:"original_URL"`
	CorrelationID string `json:"correlation_id"`
	UserID        uint64 `json:"user_id"`
	IsDeleted     bool   `json:"is_deleted"`
}

// ResponcePostURLData — элемент ответа на сокращение URL (в том числе пакетное).
type ResponcePostURLData struct {
	ShortURL      string `json:"short_URL"`
	CorrelationID string `json:"correlation_id"`
}

// ResponceUserURLData — элемент ответа GET /api/user/urls.
type ResponceUserURLData struct {
	ShortURL    string `json:"short_URL"`
	OriginalURL string `json:"original_URL"`
}

// Claims — claims JWT-токена пользователя.
type Claims struct {
	jwt.RegisteredClaims
	UserID uint64
}

// URLDataToSaveToFile — формат файла, в который сохраняется in-memory хранилище.
type URLDataToSaveToFile struct {
	CurrentUserID uint64      `json:"current_user_id"`
	URLRecords    []URLRecord `json:"URL_records"`
}

// AuditServiceMessage — сообщение аудита о действии пользователя.
type AuditServiceMessage struct {
	Timestamp   int64  `json:"ts"`
	Action      string `json:"action"`
	UserID      uint64 `json:"user_id"`
	OriginalURL string `json:"url"`
}
