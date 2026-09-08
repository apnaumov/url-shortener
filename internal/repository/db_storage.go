package repository

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"

	urlshortener "github.com/apnaumov/url-shortener.git"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type DbStorage struct {
	db *sql.DB
}

func NewDbStorage(connStr string) (*DbStorage, error) {
	err := runMigrations(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}

	dbStorage := &DbStorage{
		db: db,
	}

	return dbStorage, nil
}

func (storage *DbStorage) GetFullUrl(ctx context.Context, shortUrl string) (model.RequestURLData, error) {
	row := storage.db.QueryRowContext(ctx, getFullUrlQuery, shortUrl)

	var (
		fullUrl       string
		correlationId sql.NullString
	)

	err := row.Scan(&fullUrl, &correlationId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RequestURLData{}, NotFoundError
		}
		return model.RequestURLData{}, err
	}

	urlData := model.RequestURLData{OriginalURL: fullUrl}
	if correlationId.Valid {
		urlData.CorrelationId = correlationId.String
	}

	return urlData, nil
}

func (storage *DbStorage) SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponceURLData, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return model.ResponceURLData{}, err
	}

	defer tx.Rollback()

	responseData, err := storage.setUrlImpl(ctx, tx, urlRecord)

	txErr := tx.Commit()

	if txErr != nil {
		return model.ResponceURLData{}, err
	}

	return responseData, err
}

func (storage *DbStorage) SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponceURLData, UnacceptedUrlRecords, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	var collisionErr error = nil

	defer tx.Rollback()

	responseUrlDataBatch := make([]model.ResponceURLData, 0, len(urlRecords))
	unacceptedUrlRecords := make(UnacceptedUrlRecords, 0, 0)

	for i := range urlRecords {
		responseData, err := storage.setUrlImpl(ctx, tx, urlRecords[i])

		if err != nil {
			if errors.Is(err, FullUrlCollisionError) {
				collisionErr = FullUrlCollisionError
			} else if errors.Is(err, ShortUrlCollisionError) {
				unacceptedUrlRecords = append(unacceptedUrlRecords, urlRecords[i])
				continue
			} else {
				return nil, nil, err
			}
		}
		responseUrlDataBatch = append(responseUrlDataBatch, responseData)
	}

	txErr := tx.Commit()

	if txErr != nil {
		return nil, nil, txErr
	}

	return responseUrlDataBatch, unacceptedUrlRecords, collisionErr
}

func (storage *DbStorage) setUrlImpl(ctx context.Context, tx *sql.Tx, urlRecord model.URLRecord) (model.ResponceURLData, error) {
	shortUrlCollision := false
	row := tx.QueryRowContext(ctx, checkCollisionUrl, urlRecord.ShortURL)
	err := row.Scan(&shortUrlCollision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.ResponceURLData{}, err
	}
	if shortUrlCollision {
		return model.ResponceURLData{}, ShortUrlCollisionError
	}

	var shortUrl string
	var correlationId string
	row = tx.QueryRowContext(ctx, setFullUrlQuery, urlRecord.ShortURL, urlRecord.UrlData.OriginalURL, urlRecord.UrlData.CorrelationId)
	err = row.Scan(&shortUrl, &correlationId)
	if err != nil {
		return model.ResponceURLData{}, err
	}

	if urlRecord.ShortURL != shortUrl {
		return model.ResponceURLData{ShortUrl: shortUrl, CorrelationId: correlationId}, FullUrlCollisionError
	}

	return model.ResponceURLData{ShortUrl: urlRecord.ShortURL, CorrelationId: urlRecord.UrlData.CorrelationId}, nil
}

func (storage *DbStorage) OnServerShutdown() error {
	return storage.db.Close()
}

func (storage *DbStorage) Ping(ctx context.Context) error {
	return storage.db.PingContext(ctx)
}

func runMigrations(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	// 1. Создаём source из встроенной ФС
	source, err := iofs.New(urlshortener.MigrationsFS, "migrations")
	if err != nil {
		return err
	}

	// 2. Создаём database-драйвер из *sql.DB
	dbDriver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return err
	}

	// 3. Собираем мигратор
	m, err := migrate.NewWithInstance("iofs", source, "pgx", dbDriver)
	if err != nil {
		return err
	}
	defer m.Close()

	// 4. Применяем
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
