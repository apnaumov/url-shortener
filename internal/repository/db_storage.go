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

type DBStorage struct {
	db *sql.DB
}

func NewDBStorage(connStr string) (*DBStorage, error) {
	err := runMigrations(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}

	dbStorage := &DBStorage{
		db: db,
	}

	return dbStorage, nil
}

func (storage *DBStorage) GetFullURL(ctx context.Context, shortURL string) (string, error) {
	row := storage.db.QueryRowContext(ctx, getFullURLQuery, shortURL)

	var fullURL string

	err := row.Scan(&fullURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", NotFoundError
		}
		return "", err
	}

	return fullURL, nil
}

func (storage *DBStorage) GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error) {
	rows, err := storage.db.QueryContext(ctx, getUserURLs, userID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	usersURLData := make([]model.ResponceUserURLData, 0)

	for rows.Next() {
		var (
			shortURL string
			fullURL  string
		)
		err = rows.Scan(&shortURL, &fullURL)
		if err != nil {
			return nil, err
		}

		usersURLData = append(usersURLData, model.ResponceUserURLData{ShortURL: shortURL, OriginalURL: fullURL})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return usersURLData, nil
}

func (storage *DBStorage) SetURL(ctx context.Context, URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return model.ResponcePostURLData{}, err
	}

	defer tx.Rollback()

	responseData, err := storage.setURLImpl(ctx, tx, URLRecord)

	txErr := tx.Commit()

	if txErr != nil {
		return model.ResponcePostURLData{}, err
	}

	return responseData, err
}

func (storage *DBStorage) SetURLBatch(ctx context.Context, URLRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedURLRecords, error) {
	tx, err := storage.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	var collisionErr error = nil

	defer tx.Rollback()

	responseURLDataBatch := make([]model.ResponcePostURLData, 0, len(URLRecords))
	unacceptedURLRecords := make(UnacceptedURLRecords, 0)

	for i := range URLRecords {
		responseData, err := storage.setURLImpl(ctx, tx, URLRecords[i])

		if err != nil {
			if errors.Is(err, FullURLCollisionError) {
				collisionErr = FullURLCollisionError
			} else if errors.Is(err, ShortURLCollisionError) {
				unacceptedURLRecords = append(unacceptedURLRecords, URLRecords[i])
				continue
			} else {
				return nil, nil, err
			}
		}
		responseURLDataBatch = append(responseURLDataBatch, responseData)
	}

	txErr := tx.Commit()

	if txErr != nil {
		return nil, nil, txErr
	}

	return responseURLDataBatch, unacceptedURLRecords, collisionErr
}

func (storage *DBStorage) setURLImpl(ctx context.Context, tx *sql.Tx, URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	shortURLCollision := false
	row := tx.QueryRowContext(ctx, checkCollisionURL, URLRecord.ShortURL)
	err := row.Scan(&shortURLCollision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.ResponcePostURLData{}, err
	}
	if shortURLCollision {
		return model.ResponcePostURLData{}, ShortURLCollisionError
	}

	var shortURL string
	var correlationId string
	row = tx.QueryRowContext(ctx, setFullURLQuery, URLRecord.ShortURL, URLRecord.URLData.OriginalURL, URLRecord.URLData.CorrelationID, URLRecord.URLData.UserID)
	err = row.Scan(&shortURL, &correlationId)
	if err != nil {
		return model.ResponcePostURLData{}, err
	}

	if URLRecord.ShortURL != shortURL {
		return model.ResponcePostURLData{ShortURL: shortURL, CorrelationID: correlationId}, FullURLCollisionError
	}

	return model.ResponcePostURLData{ShortURL: URLRecord.ShortURL, CorrelationID: URLRecord.URLData.CorrelationID}, nil
}

func (storage *DBStorage) OnServerShutdown() error {
	return storage.db.Close()
}

func (storage *DBStorage) Ping(ctx context.Context) error {
	return storage.db.PingContext(ctx)
}

func (storage *DBStorage) CreateNewUser(ctx context.Context) (uint64, error) {
	var userID uint64

	row := storage.db.QueryRowContext(ctx, insertNewuserID)
	err := row.Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
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
