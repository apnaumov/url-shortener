package service

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeUsage(t *testing.T) {
	storage, err := repository.NewRuntimeStorage("")
	require.NoError(t, err)

	serv, err := NewUrlShortenerService("http://localhost:8080", storage)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	fullURL, err := serv.GetFullURL(ctx, "asd")
	assert.Empty(t, fullURL)
	assert.ErrorIs(t, err, repository.NotFoundError)

	shortURL, err := serv.SetFullURL(ctx, "asd")
	require.NoError(t, err)
	require.NotEmpty(t, shortURL)

	url, err := url.Parse(shortURL)
	require.NoError(t, err)

	fullURL, err = serv.GetFullURL(ctx, strings.ReplaceAll(url.Path, "/", ""))
	assert.NoError(t, err)
	assert.Equal(t, "asd", fullURL)
}

func TestUsageWithFileData(t *testing.T) {
	tempDir := t.TempDir()
	filepath := filepath.Join(tempDir, "test_data.storage")

	const serverBaseUrl = "http://localhost:8080"

	testData := []model.URLFileRecord{
		{ShortURL: "jhwGRw", OriginalURL: "asdasdasddsa"},
		{ShortURL: "tk7Zla", OriginalURL: "daberq"},
	}

	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	require.NoError(t, err)
	defer file.Close()

	jsonEncoder := json.NewEncoder(file)

	for _, v := range testData {
		err := jsonEncoder.Encode(v)
		require.NoError(t, err)
	}

	storage, err := repository.NewRuntimeStorage(filepath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	serv, err := NewUrlShortenerService(serverBaseUrl, storage)
	require.NoError(t, err)

	for _, v := range testData {
		fullUrl, err := serv.GetFullURL(ctx, v.ShortURL)
		assert.NoError(t, err)
		assert.Equal(t, v.OriginalURL, fullUrl)
	}
}
