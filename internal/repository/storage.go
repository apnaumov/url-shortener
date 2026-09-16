package repository

import (
	"context"
	"errors"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type URLStorage interface {
	GetFullURL(ctx context.Context, shortURL string) (string, error)
	GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error)
	SetURL(ctx context.Context, URLRecord model.URLRecord) (model.ResponcePostURLData, error)
	SetURLBatch(ctx context.Context, URLRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedURLRecords, error)
	CreateNewUser(ctx context.Context) (uint64, error)
	OnServerShutdown() error
	Ping(ctx context.Context) error
}

type UnacceptedURLRecords []model.URLRecord

var (
	ShortURLCollisionError = errors.New("storage already have this short_URL")
	NotFoundError          = errors.New("can't find record")
	FullURLCollisionError  = errors.New("storage already have this URL(s)")
)
