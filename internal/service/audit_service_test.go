package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/model"
)

func TestAuditServiceInit(t *testing.T) {
	service, err := NewAuditService(config.AuditServiceConfig{}, nil)

	assert.Error(t, err)
	assert.Nil(t, service)
}

func TestAuditServiceRemote(t *testing.T) {
	var received atomic.Int32

	remoteHandler := func(message model.AuditServiceMessage) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var receivedMessage model.AuditServiceMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedMessage))
			require.Equal(t, message.Action, receivedMessage.Action)
			require.Equal(t, message.UserID, receivedMessage.UserID)
			require.Equal(t, message.OriginalURL, receivedMessage.OriginalURL)
			require.NotEmpty(t, receivedMessage.Timestamp)
			received.Add(1)
		}
	}

	remoteServer := func(t *testing.T, handler http.HandlerFunc) string {
		srv := httptest.NewServer(handler)
		t.Cleanup(srv.Close)

		return srv.URL
	}

	t.Run("notify shorten", func(t *testing.T) {
		received.Store(0)
		expected := model.AuditServiceMessage{UserID: "1", Action: "shorten", OriginalURL: "AAA"}

		service, err := NewAuditService(config.AuditServiceConfig{URL: remoteServer(t, remoteHandler(expected))}, nil)
		require.NoError(t, err)

		require.NoError(t, service.NotifyShorten(1, expected.OriginalURL))
		require.NoError(t, service.OnServerShutdown())
		assert.EqualValues(t, 1, received.Load())
	})

	t.Run("notify follow", func(t *testing.T) {
		received.Store(0)
		expected := model.AuditServiceMessage{UserID: "1", Action: "follow", OriginalURL: "AAA"}

		service, err := NewAuditService(config.AuditServiceConfig{URL: remoteServer(t, remoteHandler(expected))}, nil)
		require.NoError(t, err)

		require.NoError(t, service.NotifyFollow(1, expected.OriginalURL))
		require.NoError(t, service.OnServerShutdown())
		assert.EqualValues(t, 1, received.Load())
	})

	t.Run("error status is reported", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		var errCount atomic.Int32
		service, err := NewAuditService(config.AuditServiceConfig{URL: srv.URL}, func(error) { errCount.Add(1) })
		require.NoError(t, err)

		require.NoError(t, service.NotifyShorten(1, "AAA"))
		require.NoError(t, service.OnServerShutdown())
		assert.EqualValues(t, 1, errCount.Load())
	})

	t.Run("notify after shutdown returns error", func(t *testing.T) {
		expected := model.AuditServiceMessage{UserID: "1", Action: "shorten", OriginalURL: "AAA"}

		service, err := NewAuditService(config.AuditServiceConfig{URL: remoteServer(t, remoteHandler(expected))}, nil)
		require.NoError(t, err)
		require.NoError(t, service.OnServerShutdown())

		assert.Error(t, service.NotifyShorten(1, "AAA"))
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

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path}, nil)
		require.NoError(t, err)

		require.NoError(t, service.NotifyShorten(1, "http://a.com"))
		require.NoError(t, service.NotifyFollow(2, "http://b.com"))
		require.NoError(t, service.OnServerShutdown())

		messages := readMessages(t, path)
		require.Len(t, messages, 2)

		assert.Equal(t, "shorten", messages[0].Action)
		assert.Equal(t, "1", messages[0].UserID)
		assert.Equal(t, "http://a.com", messages[0].OriginalURL)
		assert.NotEmpty(t, messages[0].Timestamp)

		assert.Equal(t, "follow", messages[1].Action)
		assert.Equal(t, "2", messages[1].UserID)
		assert.Equal(t, "http://b.com", messages[1].OriginalURL)
		assert.NotEmpty(t, messages[1].Timestamp)
	})

	t.Run("appends to existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.log")
		require.NoError(t, os.WriteFile(path, []byte(`{"ts":1,"action":"shorten","user_id":"9","url":"old"}`+"\n"), 0644))

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path}, nil)
		require.NoError(t, err)

		require.NoError(t, service.NotifyFollow(1, "new"))
		require.NoError(t, service.OnServerShutdown())

		messages := readMessages(t, path)
		require.Len(t, messages, 2)
		assert.Equal(t, "follow", messages[1].Action)
	})

	t.Run("concurrent writes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.log")

		service, err := NewAuditService(config.AuditServiceConfig{FilePath: path}, nil)
		require.NoError(t, err)

		const n = 50
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				assert.NoError(t, service.NotifyShorten(uint64(i), "url"))
			}(i)
		}
		wg.Wait()

		require.NoError(t, service.OnServerShutdown())
		assert.Len(t, readMessages(t, path), n)
	})
}

func TestAuditServiceFileAndRemote(t *testing.T) {
	userID := uint64(7)
	originalURL := "http://both.com"
	action := "shorten"

	checkMsg := func(msg model.AuditServiceMessage) {
		assert.Equal(t, action, msg.Action)
		assert.Equal(t, strconv.FormatUint(userID, 10), msg.UserID)
		assert.Equal(t, originalURL, msg.OriginalURL)
		assert.NotEmpty(t, msg.Timestamp)
	}

	var received atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m model.AuditServiceMessage
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&m))

		checkMsg(m)
		received.Add(1)
	}))
	t.Cleanup(srv.Close)

	path := filepath.Join(t.TempDir(), "audit.log")

	service, err := NewAuditService(config.AuditServiceConfig{FilePath: path, URL: srv.URL}, nil)
	require.NoError(t, err)
	require.Len(t, service.handlers, 2)

	require.NoError(t, service.NotifyShorten(userID, originalURL))
	require.NoError(t, service.OnServerShutdown())

	assert.EqualValues(t, 1, received.Load())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var fileMsg model.AuditServiceMessage
	require.NoError(t, json.NewDecoder(bytes.NewReader(data)).Decode(&fileMsg))

	checkMsg(fileMsg)
}
