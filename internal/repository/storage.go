package repository

import (
	"context"
	"errors"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type UrlStorage interface {
	GetFullUrl(ctx context.Context, shortUrl string) (string, error)
	GetUserUrls(ctx context.Context, userId uint64) ([]model.ResponceUserURLData, error)
	SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponcePostURLData, error)
	SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedUrlRecords, error)
	CreateNewUser(ctx context.Context) (uint64, error)
	OnServerShutdown() error
	Ping(ctx context.Context) error
}

type UnacceptedUrlRecords []model.URLRecord

var (
	ShortUrlCollisionError = errors.New("storage already have this short_url")
	NotFoundError          = errors.New("can't find record")
	FullUrlCollisionError  = errors.New("storage already have this url(s)")
)
