package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/apnaumov/url-shortener.git/internal/model"
)

type RuntimeStorage struct {
	container          *Container[model.RequestURLData]
	uniqueOriginalUrls *Container[string]
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

func (storage *RuntimeStorage) GetFullUrl(ctx context.Context, shortUrl string) (model.RequestURLData, error) {
	if ctx.Err() != nil {
		return model.RequestURLData{}, ctx.Err()
	}

	data, ok := storage.container.Get(shortUrl)

	if !ok {
		return model.RequestURLData{}, NotFoundError
	}
	return data, nil
}

func (storage *RuntimeStorage) SetUrl(ctx context.Context, urlRecord model.URLRecord) (model.ResponceURLData, error) {
	if ctx.Err() != nil {
		return model.ResponceURLData{}, ctx.Err()
	}

	return storage.setUrlImpl(urlRecord)
}

func (storage *RuntimeStorage) setUrlImpl(urlRecord model.URLRecord) (model.ResponceURLData, error) {
	existingShortUrl, ok := storage.uniqueOriginalUrls.Get(urlRecord.UrlData.OriginalURL)
	if ok {
		requestData, ok := storage.container.Get(existingShortUrl)
		if !ok {
			panic(NotFoundError)
		}

		return model.ResponceURLData{ShortUrl: existingShortUrl, CorrelationId: requestData.CorrelationId}, FullUrlCollisionError
	}

	ok = storage.container.Set(urlRecord.ShortURL, urlRecord.UrlData)
	okToServiceContainer := storage.uniqueOriginalUrls.Set(urlRecord.UrlData.OriginalURL, urlRecord.ShortURL)

	if !ok || !okToServiceContainer {
		return model.ResponceURLData{}, ShortUrlCollisionError
	}

	return model.ResponceURLData{ShortUrl: urlRecord.ShortURL, CorrelationId: urlRecord.UrlData.CorrelationId}, nil
}

func (storage *RuntimeStorage) SetUrlBatch(ctx context.Context, urlRecords []model.URLRecord) ([]model.ResponceURLData, UnacceptedUrlRecords, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	unacceptedUrlRecords := make(UnacceptedUrlRecords, 0, 0)
	responseUrlDataBatch := make([]model.ResponceURLData, 0, len(urlRecords))
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

func (storage *RuntimeStorage) loadFromFile() error {
	file, err := os.OpenFile(storage.fileStoragePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)

	for {
		record := model.URLRecord{}
		err := jsonDecoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if ok := storage.container.Set(record.ShortURL, record.UrlData); !ok {
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

	for k, v := range storageContainer {
		if err := jsonEncoder.Encode(model.URLRecord{ShortURL: k, UrlData: v}); err != nil {
			return err
		}
	}

	return nil
}
