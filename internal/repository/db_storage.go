package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	urlshortener "github.com/apnaumov/url-shortener.git"
	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type DBStorage struct {
	db                      *sql.DB
	logger                  *zap.Logger
	pendingMessageProcessor *PendingMessageProcessor[DeleteUserURLsDTO]
}

func NewDBStorage(connStr string, messageProcessorConfig *config.PendingMessageProcessorConfig, logger *zap.Logger) (*DBStorage, error) {
	err := runMigrations(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, err
	}

	dbStorage := &DBStorage{
		db:     db,
		logger: logger,
	}

	dbStorage.pendingMessageProcessor = NewPendingMessageProcessor(
		time.Duration(messageProcessorConfig.TickTime)*time.Second,
		messageProcessorConfig.BufferSize,
		messageProcessorConfig.MaxBatchSize,
		messageProcessorConfig.WorkerPoolSize,
		messageProcessorConfig.MaxParallelInsertsToQueue,
		dbStorage.deleteURLsTickFunc,
		logger)

	dbStorage.pendingMessageProcessor.Run()

	return dbStorage, nil
}

// Возвращаем только ошибки, касающиеся БД, чтобы повторить операцию в PendingMessageProcessor
func (storage *DBStorage) deleteURLsTickFunc(messages []DeleteUserURLsDTO) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var paramsForDeleteUrlsQuery struct {
		counter      int
		placeholders []string
		args         []any
	}

	for i := range messages {
		placeholders := make([]string, len(messages[i].ShortURLs))
		args := make([]any, len(messages[i].ShortURLs))
		for j, u := range messages[i].ShortURLs {
			placeholders[j] = fmt.Sprintf("$%d", j+1)
			args[j] = u

			paramsForDeleteUrlsQuery.placeholders = append(paramsForDeleteUrlsQuery.placeholders, fmt.Sprintf("$%d", paramsForDeleteUrlsQuery.counter+1))
			paramsForDeleteUrlsQuery.args = append(paramsForDeleteUrlsQuery.args, u)
			paramsForDeleteUrlsQuery.counter++
		}

		resQueryToGet := fmt.Sprintf(
			getURLRecordsForDelete,
			strings.Join(placeholders, ","),
		)

		rows, err := storage.db.QueryContext(ctx, resQueryToGet, args...)

		if err != nil {
			return err
		}

		defer rows.Close()

		countRows := 0
		for rows.Next() {
			var (
				userID    uint64
				isDeleted bool
			)
			err = rows.Scan(&userID, &isDeleted)
			if err != nil {
				return err
			}

			if userID != messages[i].UserID {
				storage.logger.Error(ErrDeleteProhibited.Error(), zap.Uint64("Current UserID", userID), zap.Uint64("Request UserID", messages[i].UserID))
				return nil
			}

			if isDeleted {
				storage.logger.Error(ErrDeleted.Error())
				return nil
			}

			countRows++
		}

		if err := rows.Err(); err != nil {
			return err
		}

		if countRows != len(messages[i].ShortURLs) {
			storage.logger.Error(ErrNotFound.Error())
			return nil
		}
	}

	resDeleteQuery := fmt.Sprintf(
		setDeletedURL,
		strings.Join(paramsForDeleteUrlsQuery.placeholders, ","),
	)

	_, err = tx.ExecContext(ctx, resDeleteQuery, paramsForDeleteUrlsQuery.args...)

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (storage *DBStorage) GetFullURL(ctx context.Context, shortURL string) (string, error) {
	row := storage.db.QueryRowContext(ctx, getFullURLQuery, shortURL)

	var fullURL string
	var isDeleted bool

	err := row.Scan(&fullURL, &isDeleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	if isDeleted {
		return "", ErrDeleted
	}

	return fullURL, nil
}

func (storage *DBStorage) GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error) {
	rows, err := storage.db.QueryContext(ctx, getFilteredByDeleteUserURLs, userID, false)

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

func (storage *DBStorage) DeleteUserURLs(deleteUserURLs DeleteUserURLsDTO) {
	storage.pendingMessageProcessor.InsertToQueue(deleteUserURLs)
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
			if errors.Is(err, ErrFullURLCollision) {
				collisionErr = ErrFullURLCollision
			} else if errors.Is(err, ErrShortURLCollision) {
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
		return model.ResponcePostURLData{}, ErrShortURLCollision
	}

	var shortURL string
	var correlationID string
	row = tx.QueryRowContext(ctx, setFullURLQuery, URLRecord.ShortURL, URLRecord.URLData.OriginalURL, URLRecord.URLData.CorrelationID, URLRecord.URLData.UserID)
	err = row.Scan(&shortURL, &correlationID)
	if err != nil {
		return model.ResponcePostURLData{}, err
	}

	if URLRecord.ShortURL != shortURL {
		return model.ResponcePostURLData{ShortURL: shortURL, CorrelationID: correlationID}, ErrFullURLCollision
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

	row := storage.db.QueryRowContext(ctx, insertNewUserID)
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
