package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type RuntimeStorage struct {
	container          *Container[model.RequestURLData]
	uniqueOriginalUrls *Container[string]
	currentUserId      atomic.Uint64
	fileStoragePath    string
}

func NewRuntimeStorage(fileStoragePath string) (*RuntimeStorage, error) {
	storage := &RuntimeStorage{
		container:          NewContainer[model.RequestURLData](),
		uniqueOriginalUrls: NewContainer[string](),
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

func (storage *RuntimeStorage) GetFullUrl(ctx context.Context, shortUrl string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	data, ok := storage.container.Get(shortUrl)

	if !ok {
		return "", NotFoundError
	}
	return data.OriginalURL, nil
}

func (storage *RuntimeStorage) GetUserUrls(ctx context.Context, userId uint64) ([]model.ResponceUserURLData, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	userData := make([]model.ResponceUserURLData, 0)

	storageRecords := storage.container.GetAll()
	for i, v := range storageRecords {
		if v.UserId == userId {
			userData = append(userData, model.ResponceUserURLData{ShortUrl: i, OriginalURL: v.OriginalURL})
		}
	}

	return userData, nil
}

func (storage *RuntimeStorage) SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponcePostURLData, error) {
	if ctx.Err() != nil {
		return model.ResponcePostURLData{}, ctx.Err()
	}

	return storage.setUrlImpl(urlRecord)
}

func (storage *RuntimeStorage) setUrlImpl(urlRecord model.URLRecord) (model.ResponcePostURLData, error) {
	existingShortUrl, ok := storage.uniqueOriginalUrls.Get(urlRecord.UrlData.OriginalURL)
	if ok {
		requestData, ok := storage.container.Get(existingShortUrl)
		if !ok {
			panic(NotFoundError)
		}

		return model.ResponcePostURLData{ShortUrl: existingShortUrl, CorrelationId: requestData.CorrelationId}, FullUrlCollisionError
	}

	ok = storage.container.Set(urlRecord.ShortURL, urlRecord.UrlData)
	okToServiceContainer := storage.uniqueOriginalUrls.Set(urlRecord.UrlData.OriginalURL, urlRecord.ShortURL)

	if !ok || !okToServiceContainer {
		return model.ResponcePostURLData{}, ShortUrlCollisionError
	}

	return model.ResponcePostURLData{ShortUrl: urlRecord.ShortURL, CorrelationId: urlRecord.UrlData.CorrelationId}, nil
}

func (storage *RuntimeStorage) SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponcePostURLData, UnacceptedUrlRecords, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	unacceptedUrlRecords := make(UnacceptedUrlRecords, 0)
	responseUrlDataBatch := make([]model.ResponcePostURLData, 0, len(urlRecords))
	var collisionErr error = nil

	for i := range urlRecords {
		responseData, err := storage.setUrlImpl(urlRecords[i])

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
	return responseUrlDataBatch, unacceptedUrlRecords, collisionErr
}

func (storage *RuntimeStorage) OnServerShutdown() error {
	return storage.saveToFile()
}

func (storage *RuntimeStorage) Ping(ctx context.Context) error {
	return nil
}

func (storage *RuntimeStorage) CreateNewUser(ctx context.Context) (uint64, error) {
	userId := storage.currentUserId.Add(1)
	return userId, nil
}

func (storage *RuntimeStorage) loadFromFile() error {
	file, err := os.OpenFile(storage.fileStoragePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)

	fileData := model.UrlDataToSaveToFile{}
	err = jsonDecoder.Decode(&fileData)

	if err != nil {
		return err
	}

	storage.currentUserId.Store(fileData.CurrentUserId)
	for i := range fileData.UrlRecords {
		if ok := storage.container.Set(fileData.UrlRecords[i].ShortURL, fileData.UrlRecords[i].UrlData); !ok {
			return FullUrlCollisionError
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

	urlRecords := make([]model.URLRecord, 0, len(storageContainer))

	for k, v := range storageContainer {
		urlRecords = append(urlRecords, model.URLRecord{ShortURL: k, UrlData: v})
	}

	fileData := model.UrlDataToSaveToFile{}
	fileData.CurrentUserId = storage.currentUserId.Load()
	fileData.UrlRecords = urlRecords

	if err := jsonEncoder.Encode(fileData); err != nil {
		return err
	}

	return nil
}
