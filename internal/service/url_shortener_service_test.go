package service

import (
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
	serv, err := NewUrlShortenerService("http://localhost:8080", "")
	require.NoError(t, err)

	fullURL, err := serv.GetFullURL("asd")
	assert.Empty(t, fullURL)
	assert.ErrorIs(t, err, repository.NotFoundError)

	shortURL, err := serv.SetFullURL("asd")
	require.NoError(t, err)
	require.NotEmpty(t, shortURL)

	url, err := url.Parse(shortURL)
	require.NoError(t, err)

	fullURL, err = serv.GetFullURL(strings.ReplaceAll(url.Path, "/", ""))
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

	serv, err := NewUrlShortenerService(serverBaseUrl, filepath)
	require.NoError(t, err)

	for _, v := range testData {
		fullUrl, err := serv.GetFullURL(v.ShortURL)
		assert.NoError(t, err)
		assert.Equal(t, v.OriginalURL, fullUrl)
	}
}
