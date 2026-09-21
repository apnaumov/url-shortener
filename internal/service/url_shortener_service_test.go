package service

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeUsage(t *testing.T) {
	logger, err := logger.InitializeRootLogger("test", "debug")
	require.NoError(t, err)

	storage, err := repository.NewRuntimeStorage("", logger)
	require.NoError(t, err)

	serv, err := NewURLShortenerService("http://localhost:8080", storage)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	URLData, err := serv.GetFullURL(ctx, "asd")
	assert.Empty(t, URLData)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	responseData, err := serv.SetFullURL(ctx, model.RequestURLData{OriginalURL: "asd"})
	require.NoError(t, err)
	require.NotEmpty(t, responseData.ShortURL)

	URL, err := url.Parse(responseData.ShortURL)
	require.NoError(t, err)

	URLData, err = serv.GetFullURL(ctx, strings.ReplaceAll(URL.Path, "/", ""))
	assert.NoError(t, err)
	assert.Equal(t, "asd", URLData)
}

func TestUsageWithFileData(t *testing.T) {
	tempDir := t.TempDir()
	filepath := filepath.Join(tempDir, "test_data.storage")

	const serverBaseURL = "http://localhost:8080"

	testData := model.URLDataToSaveToFile{
		CurrentUserID: 0,
		URLRecords: []model.URLRecord{
			{ShortURL: "jhwGRw", URLData: model.RequestURLData{OriginalURL: "asdasdasddsa"}},
			{ShortURL: "tk7Zla", URLData: model.RequestURLData{OriginalURL: "daberq"}},
		},
	}

	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	require.NoError(t, err)
	defer file.Close()

	jsonEncoder := json.NewEncoder(file)

	err = jsonEncoder.Encode(testData)
	require.NoError(t, err)

	logger, err := logger.InitializeRootLogger("test", "debug")
	require.NoError(t, err)

	storage, err := repository.NewRuntimeStorage(filepath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	serv, err := NewURLShortenerService(serverBaseURL, storage)
	require.NoError(t, err)

	for _, v := range testData.URLRecords {
		fullURL, err := serv.GetFullURL(ctx, v.ShortURL)
		assert.NoError(t, err)
		assert.Equal(t, v.URLData.OriginalURL, fullURL)
	}
}
