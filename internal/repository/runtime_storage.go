package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
)

// RuntimeStorage — in-memory хранилище URL с опциональным сохранением в файл. Реализует URLStorage.
type RuntimeStorage struct {
	container               *Container[model.RequestURLData]
	uniqueOriginalURLs      *Container[string]
	currentUserID           atomic.Uint64
	fileStoragePath         string
	pendingMessageProcessor *PendingMessageProcessor[DeleteUserURLsDTO]
	logger                  *zap.Logger
}

// NewRuntimeStorage создаёт хранилище и загружает данные из fileStoragePath, если он задан.
func NewRuntimeStorage(fileStoragePath string, messageProcessorConfig *config.PendingMessageProcessorConfig, logger *zap.Logger) (*RuntimeStorage, error) {
	storage := &RuntimeStorage{
		container:          NewContainer[model.RequestURLData](),
		uniqueOriginalURLs: NewContainer[string](),
		fileStoragePath:    fileStoragePath,
		logger:             logger,
	}

	if len(fileStoragePath) != 0 {
		err := storage.loadFromFile()
		if err != nil {
			return nil, err
		}
	}

	storage.pendingMessageProcessor = NewPendingMessageProcessor(
		time.Duration(messageProcessorConfig.TickTime)*time.Second,
		messageProcessorConfig.BufferSize,
		messageProcessorConfig.MaxBatchSize,
		messageProcessorConfig.WorkerPoolSize,
		messageProcessorConfig.MaxParallelInsertsToQueue,
		storage.deleteURLsTickFunc,
		logger)

	storage.pendingMessageProcessor.Run()

	return storage, nil
}

// deleteURLsTickFunc помечает URL из пачки сообщений удалёнными.
func (storage *RuntimeStorage) deleteURLsTickFunc(messages []DeleteUserURLsDTO) error {
	for i := range messages {
		for j := range messages[i].ShortURLs {
			elem, ok := storage.container.Get(messages[i].ShortURLs[j])

			if !ok {
				storage.logger.Error(ErrNotFound.Error())
				return nil
			}

			if elem.UserID != messages[i].UserID {
				storage.logger.Error(ErrDeleteProhibited.Error())
				return nil
			}

			if elem.IsDeleted {
				storage.logger.Error(ErrDeleted.Error())
				return nil
			}

			elem.IsDeleted = true
			storage.container.ForceSet(messages[i].ShortURLs[j], elem)
		}
	}

	return nil
}

// GetFullURL возвращает исходный URL по короткому ключу.
// Возвращает ErrNotFound или ErrDeleted, если записи нет или она удалена.
func (storage *RuntimeStorage) GetFullURL(ctx context.Context, shortURL string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	data, ok := storage.container.Get(shortURL)

	if !ok {
		return "", ErrNotFound
	}

	if data.IsDeleted {
		return "", ErrDeleted
	}

	return data.OriginalURL, nil
}

// GetUserURLs возвращает неудалённые URL пользователя.
func (storage *RuntimeStorage) GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	userData := make([]model.ResponceUserURLData, 0)

	storage.container.Range(func(shortURL string, v model.RequestURLData) {
		if v.UserID == userID && !v.IsDeleted {
			userData = append(userData, model.ResponceUserURLData{ShortURL: shortURL, OriginalURL: v.OriginalURL})
		}
	})

	return userData, nil
}

// DeleteUserURLs ставит удаление URL в очередь на асинхронную обработку.
func (storage *RuntimeStorage) DeleteUserURLs(deleteUserURLs DeleteUserURLsDTO) {
	storage.pendingMessageProcessor.InsertToQueue(deleteUserURLs)
}

// SetURL сохраняет запись. Возвращает ErrShortURLCollision, если короткий ключ занят,
// и ErrFullURLCollision (вместе с существующей записью), если такой URL уже сокращён.
func (storage *RuntimeStorage) SetURL(ctx context.Context, URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	if ctx.Err() != nil {
		return model.ResponcePostURLData{}, ctx.Err()
	}

	return storage.setURLImpl(URLRecord)
}

// setURLImpl сохраняет запись без проверки контекста.
func (storage *RuntimeStorage) setURLImpl(URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	existingShortURL, ok := storage.uniqueOriginalURLs.Get(URLRecord.URLData.OriginalURL)
	if ok {
		requestData, ok := storage.container.Get(existingShortURL)
		if !ok {
			return model.ResponcePostURLData{}, ErrNotFound
		}

		return model.ResponcePostURLData{ShortURL: existingShortURL, CorrelationID: requestData.CorrelationID}, ErrFullURLCollision
	}

	ok = storage.container.Set(URLRecord.ShortURL, URLRecord.URLData)
	okToServiceContainer := storage.uniqueOriginalURLs.Set(URLRecord.URLData.OriginalURL, URLRecord.ShortURL)

	if !ok || !okToServiceContainer {
		return model.ResponcePostURLData{}, ErrShortURLCollision
	}

	return model.ResponcePostURLData{ShortURL: URLRecord.ShortURL, CorrelationID: URLRecord.URLData.CorrelationID}, nil
}

// SetURLBatch сохраняет пачку записей. Записи с занятым коротким ключом возвращаются
// в UnacceptedURLRecords, при дубликатах URL — ErrFullURLCollision.
func (storage *RuntimeStorage) SetURLBatch(ctx context.Context, URLRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedURLRecords, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	unacceptedURLRecords := make(UnacceptedURLRecords, 0)
	responseURLDataBatch := make([]model.ResponcePostURLData, 0, len(URLRecords))
	var collisionErr error = nil

	for i := range URLRecords {
		responseData, err := storage.setURLImpl(URLRecords[i])

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
	return responseURLDataBatch, unacceptedURLRecords, collisionErr
}

// OnServerShutdown останавливает обработку очереди и сохраняет данные в файл.
func (storage *RuntimeStorage) OnServerShutdown() error {
	storage.pendingMessageProcessor.Shutdown()
	return storage.saveToFile()
}

// Ping всегда возвращает nil: in-memory хранилище доступно всегда.
func (storage *RuntimeStorage) Ping(ctx context.Context) error {
	return nil
}

// CreateNewUser выдаёт следующий ID пользователя.
func (storage *RuntimeStorage) CreateNewUser(ctx context.Context) (uint64, error) {
	userID := storage.currentUserID.Add(1)
	return userID, nil
}

// loadFromFile загружает данные из файла хранилища (файл создаётся, если его нет).
func (storage *RuntimeStorage) loadFromFile() error {
	file, err := os.OpenFile(storage.fileStoragePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)

	fileData := model.URLDataToSaveToFile{}
	err = jsonDecoder.Decode(&fileData)

	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	storage.currentUserID.Store(fileData.CurrentUserID)
	for i := range fileData.URLRecords {
		if ok := storage.container.Set(fileData.URLRecords[i].ShortURL, fileData.URLRecords[i].URLData); !ok {
			return ErrFullURLCollision
		}
	}

	return nil
}

// saveToFile записывает все данные в файл хранилища, если путь к нему задан.
func (storage *RuntimeStorage) saveToFile() error {
	if len(storage.fileStoragePath) == 0 {
		return nil
	}

	file, err := os.OpenFile(storage.fileStoragePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	storageContainer := storage.container.GetAll()

	jsonEncoder := json.NewEncoder(file)

	URLRecords := make([]model.URLRecord, 0, len(storageContainer))

	for k, v := range storageContainer {
		URLRecords = append(URLRecords, model.URLRecord{ShortURL: k, URLData: v})
	}

	fileData := model.URLDataToSaveToFile{}
	fileData.CurrentUserID = storage.currentUserID.Load()
	fileData.URLRecords = URLRecords

	if err := jsonEncoder.Encode(fileData); err != nil {
		return err
	}

	return nil
}
