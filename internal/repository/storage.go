package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/apnaumov/url-shortener.git/internal/model"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	urlshortener "github.com/apnaumov/url-shortener.git"
)

type UrlStorage interface {
	GetFullUrl(ctx context.Context, shortUrl string) (string, error)
	SetUrl(ctx context.Context, shortUrl string, fullUrl string) error
	OnServerShutdown() error
}

var (
	NotFoundError  = errors.New("doesn't have in storage")
	CollisionError = errors.New("storage already have this key")
)

type RuntimeStorage struct {
	container       *Container[string]
	fileStoragePath string
}

func NewRuntimeStorage(fileStoragePath string) (*RuntimeStorage, error) {
	storage := &RuntimeStorage{
		container:       NewContainer[string](),
		fileStoragePath: fileStoragePath,
	}

	if len(fileStoragePath) != 0 {
		err := storage.loadFromFile()
		if err != nil {
			return nil, err
		}
	}

	return storage, nil
}

func (storage *RuntimeStorage) GetFullUrl(ctx context.Context, shortUrl string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	return storage.container.Get(shortUrl)
}

func (storage *RuntimeStorage) SetUrl(ctx context.Context, shortUrl string, fullUrl string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return storage.container.Set(shortUrl, fullUrl)
}

func (storage *RuntimeStorage) OnServerShutdown() error {
	return storage.saveToFile()
}

func (storage *RuntimeStorage) loadFromFile() error {
	file, err := os.OpenFile(storage.fileStoragePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)

	for {
		record := model.URLFileRecord{}
		err := jsonDecoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := storage.container.Set(record.ShortURL, record.OriginalURL); err != nil {
			return err
		}
	}

	return nil
}

func (storage *RuntimeStorage) saveToFile() error {
	file, err := os.OpenFile(storage.fileStoragePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	storageContainer := storage.container.GetAll()

	jsonEncoder := json.NewEncoder(file)

	for k, v := range storageContainer {
		if err := jsonEncoder.Encode(model.URLFileRecord{ShortURL: k, OriginalURL: v}); err != nil {
			return err
		}
	}

	return nil
}

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

func (storage *DbStorage) GetNativeDb() *sql.DB {
	return storage.db
}

func (storage *DbStorage) GetFullUrl(ctx context.Context, shortUrl string) (string, error) {
	var fullUrl string
	row := storage.db.QueryRowContext(ctx, getFullUrlQuery, shortUrl)

	err := row.Scan(&fullUrl)

	if err != nil {
		return "", err
	}

	return fullUrl, nil
}

func (storage *DbStorage) SetUrl(ctx context.Context, shortUrl string, fullUrl string) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	isExist := false
	row := tx.QueryRowContext(ctx, checkShortUrlIsExistQuery, shortUrl)

	err = row.Scan(&isExist)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if isExist {
		return CollisionError
	}

	_, err = tx.ExecContext(ctx, setFullUrlQuery, shortUrl, fullUrl)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (storage *DbStorage) OnServerShutdown() error {
	return storage.db.Close()
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
