package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditServiceInit(t *testing.T) {
	service, err := NewAuditService(config.AuditServiceConfig{})

	assert.Error(t, err)
	assert.Nil(t, service)
}

func TestAuditServiceRemote(t *testing.T) {
	remoteHandler := func(message model.AuditServiceMessage) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var receivedMessage model.AuditServiceMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedMessage))
			require.Equal(t, message.Action, receivedMessage.Action)
			require.Equal(t, message.UserID, receivedMessage.UserID)
			require.Equal(t, message.OriginalURL, receivedMessage.OriginalURL)
			require.NotEmpty(t, receivedMessage.Timestamp)
		}
	}

	remoteServer := func(t *testing.T, handler http.HandlerFunc) string {
		srv := httptest.NewServer(handler)
		t.Cleanup(srv.Close)
		return srv.URL
	}

	t.Run("notify shorten", func(t *testing.T) {
		expected := model.AuditServiceMessage{UserID: 1, Action: "shorten", OriginalURL: "AAA"}

		url := remoteServer(t, remoteHandler(expected))

		service, err := NewAuditService(config.AuditServiceConfig{URL: url})
		require.NoError(t, err)

		require.Empty(t, service.NotifyShorten(expected.UserID, expected.OriginalURL))
	})

	t.Run("notify follow", func(t *testing.T) {
		expected := model.AuditServiceMessage{UserID: 1, Action: "follow", OriginalURL: "AAA"}

		url := remoteServer(t, remoteHandler(expected))

		service, err := NewAuditService(config.AuditServiceConfig{URL: url})
		require.NoError(t, err)

		require.Empty(t, service.NotifyFollow(expected.UserID, expected.OriginalURL))
	})

}

func TestAuditServiceFile(t *testing.T) {
	readMessages := func(t *testing.T, path string) []model.AuditServiceMessage {
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var messages []model.AuditServiceMessage
		dec := json.NewDecoder(bytes.NewReader(data))
		for dec.More() {
			var m model.AuditServiceMessage
			require.NoError(t, dec.Decode(&m))
			messages = append(messages, m)
		}
		return messages
	}

	t.Run("notify shorten and follow write to file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.log")

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path})
		require.NoError(t, err)

		require.Empty(t, service.NotifyShorten(1, "http://a.com"))
		require.Empty(t, service.NotifyFollow(2, "http://b.com"))

		messages := readMessages(t, path)
		require.Len(t, messages, 2)

		assert.Equal(t, "shorten", messages[0].Action)
		assert.Equal(t, uint64(1), messages[0].UserID)
		assert.Equal(t, "http://a.com", messages[0].OriginalURL)
		assert.NotEmpty(t, messages[0].Timestamp)

		assert.Equal(t, "follow", messages[1].Action)
		assert.Equal(t, uint64(2), messages[1].UserID)
		assert.Equal(t, "http://b.com", messages[1].OriginalURL)
		assert.NotEmpty(t, messages[1].Timestamp)
	})

	t.Run("appends to existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.log")
		require.NoError(t, os.WriteFile(path, []byte(`{"ts":1,"action":"shorten","user_id":9,"url":"old"}`+"\n"), 0644))

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path})
		require.NoError(t, err)

		require.Empty(t, service.NotifyFollow(1, "new"))

		messages := readMessages(t, path)
		require.Len(t, messages, 2)
		assert.Equal(t, "follow", messages[1].Action)
	})

	t.Run("concurrent writes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.log")

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path})
		require.NoError(t, err)

		const n = 50
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				assert.Empty(t, service.NotifyShorten(uint64(i), "url"))
			}(i)
		}
		wg.Wait()

		assert.Len(t, readMessages(t, path), n)
	})
}

func TestAuditServiceFileAndRemote(t *testing.T) {
	userID := uint64(7)
	originalURL := "http://both.com"
	action := "shorten"

	checkMsg := func(msg model.AuditServiceMessage) {
		assert.Equal(t, action, msg.Action)
		assert.Equal(t, userID, msg.UserID)
		assert.Equal(t, originalURL, msg.OriginalURL)
		assert.NotEmpty(t, msg.Timestamp)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m model.AuditServiceMessage
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&m))

		checkMsg(m)
	}))
	t.Cleanup(srv.Close)

	path := filepath.Join(t.TempDir(), "audit.log")

	service, err := NewAuditService(config.AuditServiceConfig{FilePath: path, URL: srv.URL})
	require.NoError(t, err)
	require.Len(t, service.handlers, 2)

	require.Empty(t, service.NotifyShorten(userID, originalURL))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var fileMsg model.AuditServiceMessage
	require.NoError(t, json.NewDecoder(bytes.NewReader(data)).Decode(&fileMsg))

	checkMsg(fileMsg)
}
