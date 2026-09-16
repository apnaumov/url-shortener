package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type RuntimeStorage struct {
	container          *Container[model.RequestURLData]
	uniqueOriginalURLs *Container[string]
	currentUserID      atomic.Uint64
	fileStoragePath    string
}

func NewRuntimeStorage(fileStoragePath string) (*RuntimeStorage, error) {
	storage := &RuntimeStorage{
		container:          NewContainer[model.RequestURLData](),
		uniqueOriginalURLs: NewContainer[string](),
		fileStoragePath:    fileStoragePath,
	}

	if len(fileStoragePath) != 0 {
		err := storage.loadFromFile()
		if err != nil {
			return nil, err
		}
	}

	return storage, nil
}

func (storage *RuntimeStorage) GetFullURL(ctx context.Context, shortURL string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	data, ok := storage.container.Get(shortURL)

	if !ok {
		return "", NotFoundError
	}
	return data.OriginalURL, nil
}

func (storage *RuntimeStorage) GetUserURLs(ctx context.Context, userID uint64) ([]model.ResponceUserURLData, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	userData := make([]model.ResponceUserURLData, 0)

	storageRecords := storage.container.GetAll()
	for i, v := range storageRecords {
		if v.UserID == userID {
			userData = append(userData, model.ResponceUserURLData{ShortURL: i, OriginalURL: v.OriginalURL})
		}
	}

	return userData, nil
}

func (storage *RuntimeStorage) SetURL(ctx context.Context, URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	if ctx.Err() != nil {
		return model.ResponcePostURLData{}, ctx.Err()
	}

	return storage.setURLImpl(URLRecord)
}

func (storage *RuntimeStorage) setURLImpl(URLRecord model.URLRecord) (model.ResponcePostURLData, error) {
	existingShortURL, ok := storage.uniqueOriginalURLs.Get(URLRecord.URLData.OriginalURL)
	if ok {
		requestData, ok := storage.container.Get(existingShortURL)
		if !ok {
			panic(NotFoundError)
		}

		return model.ResponcePostURLData{ShortURL: existingShortURL, CorrelationID: requestData.CorrelationID}, FullURLCollisionError
	}

	ok = storage.container.Set(URLRecord.ShortURL, URLRecord.URLData)
	okToServiceContainer := storage.uniqueOriginalURLs.Set(URLRecord.URLData.OriginalURL, URLRecord.ShortURL)

	if !ok || !okToServiceContainer {
		return model.ResponcePostURLData{}, ShortURLCollisionError
	}

	return model.ResponcePostURLData{ShortURL: URLRecord.ShortURL, CorrelationID: URLRecord.URLData.CorrelationID}, nil
}

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
	return responseURLDataBatch, unacceptedURLRecords, collisionErr
}

func (storage *RuntimeStorage) OnServerShutdown() error {
	return storage.saveToFile()
}

func (storage *RuntimeStorage) Ping(ctx context.Context) error {
	return nil
}

func (storage *RuntimeStorage) CreateNewUser(ctx context.Context) (uint64, error) {
	userID := storage.currentUserID.Add(1)
	return userID, nil
}

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
			return FullURLCollisionError
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
