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

func (storage *DbStorage) GetFullUrl(ctx context.Context, shortUrl string) (string, error) {
	row := storage.db.QueryRowContext(ctx, getFullUrlQuery, shortUrl)

	var fullUrl string

	err := row.Scan(&fullUrl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", NotFoundError
		}
		return "", err
	}

	return fullUrl, nil
}

func (storage *DbStorage) GetUserUrls(ctx context.Context, userId uint64) ([]model.ResponceUserURLData, error) {
	rows, err := storage.db.QueryContext(ctx, getUserUrls, userId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	usersUrlData := make([]model.ResponceUserURLData, 0, 0)

	for rows.Next() {
		var (
			shortUrl string
			fullUrl  string
		)
		err = rows.Scan(&shortUrl, &fullUrl)
		if err != nil {
			return nil, err
		}

		usersUrlData = append(usersUrlData, model.ResponceUserURLData{ShortUrl: shortUrl, OriginalURL: fullUrl})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return usersUrlData, nil
}

func (storage *DbStorage) SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponcePostURLData, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return model.ResponcePostURLData{}, err
	}

	defer tx.Rollback()

	responseData, err := storage.setUrlImpl(ctx, tx, urlRecord)

	txErr := tx.Commit()

	if txErr != nil {
		return model.ResponcePostURLData{}, err
	}

	return responseData, err
}

func (storage *DbStorage) SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedUrlRecords, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	var collisionErr error = nil

	defer tx.Rollback()

	responseUrlDataBatch := make([]model.ResponcePostURLData, 0, len(urlRecords))
	unacceptedUrlRecords := make(UnacceptedUrlRecords, 0)

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

func (storage *DbStorage) setUrlImpl(ctx context.Context, tx *sql.Tx, urlRecord model.URLRecord) (model.ResponcePostURLData, error) {
	shortUrlCollision := false
	row := tx.QueryRowContext(ctx, checkCollisionUrl, urlRecord.ShortURL)
	err := row.Scan(&shortUrlCollision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.ResponcePostURLData{}, err
	}
	if shortUrlCollision {
		return model.ResponcePostURLData{}, ShortUrlCollisionError
	}

	var shortUrl string
	var correlationId string
	row = tx.QueryRowContext(ctx, setFullUrlQuery, urlRecord.ShortURL, urlRecord.UrlData.OriginalURL, urlRecord.UrlData.CorrelationId, urlRecord.UrlData.UserId)
	err = row.Scan(&shortUrl, &correlationId)
	if err != nil {
		return model.ResponcePostURLData{}, err
	}

	if urlRecord.ShortURL != shortUrl {
		return model.ResponcePostURLData{ShortUrl: shortUrl, CorrelationId: correlationId}, FullUrlCollisionError
	}

	return model.ResponcePostURLData{ShortUrl: urlRecord.ShortURL, CorrelationId: urlRecord.UrlData.CorrelationId}, nil
}

func (storage *DbStorage) OnServerShutdown() error {
	return storage.db.Close()
}

func (storage *DbStorage) Ping(ctx context.Context) error {
	return storage.db.PingContext(ctx)
}

func (storage *DbStorage) CreateNewUser(ctx context.Context) (uint64, error) {
	var userId uint64

	row := storage.db.QueryRowContext(ctx, insertNewUserId)
	err := row.Scan(&userId)
	if err != nil {
		return 0, err
	}

	return userId, nil
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
