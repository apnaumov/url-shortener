package repository

import (
	"context"
	"errors"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type UrlStorage interface {
	GetFullUrl(ctx context.Context, shortUrl string) (model.RequestURLData, error)
	SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponceURLData, error)
	SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponceURLData, UnacceptedUrlRecords, error)
	OnServerShutdown() error
	Ping(ctx context.Context) error
}

type UnacceptedUrlRecords []model.URLRecord

var (
	ShortUrlCollisionError = errors.New("storage already have this short_url")
	NotFoundError          = errors.New("can't find record")
	FullUrlCollisionError  = errors.New("storage already have this url(s)")
)
