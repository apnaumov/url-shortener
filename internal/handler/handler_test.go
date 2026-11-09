package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setUpServer(t *testing.T) *httptest.Server {
	t.Helper()
	return setUpServerWithAudit(t, config.AuditServiceConfig{})
}

func setUpServerWithAudit(t *testing.T, auditConf config.AuditServiceConfig) *httptest.Server {
	t.Helper()
	// получение URL и последующая настройка
	ts := httptest.NewUnstartedServer(nil)

	pendingMessageProcessorParams := config.PendingMessageProcessorConfig{
		TickTime:                  10,
		BufferSize:                1024,
		MaxBatchSize:              512,
		WorkerPoolSize:            20,
		MaxParallelInsertsToQueue: 20,
	}

	logger, err := logger.InitializeRootLogger("test", "debug")
	require.NoError(t, err)
	storage, err := repository.NewRuntimeStorage("", &pendingMessageProcessorParams, logger)

	require.NoError(t, err)

	router, err := NewURLShortenerRouter("http://"+ts.Listener.Addr().String(), []byte("mysupersecretkey"), storage, auditConf)
	require.NoError(t, err)
	ts.Config.Handler = router.Mux

	return ts
}

func TestRouterForMethodNotAllowed(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/", strings.NewReader("Example body"))
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "")

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Method not allowed\n", string(buf))
}

func TestPostNewURL(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	type postMethod struct {
		prefix      string
		contentType string
		body        string
	}

	type want struct {
		code          int
		bodyNotEmpty  bool
		preferredBody string
		contentType   string
	}
	tests := []struct {
		name       string
		postMethod postMethod
		want       want
	}{
		{
			name: "positive test",
			postMethod: postMethod{
				prefix:      "",
				contentType: "text/plain",
				body:        "http://abc.abc",
			},
			want: want{
				code:         http.StatusCreated,
				bodyNotEmpty: true,
				contentType:  "text/plain",
			},
		},
		{
			name: "wrong content type test",
			postMethod: postMethod{
				prefix:      "",
				contentType: "application/json",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "Content-type incorrect\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "body must be not empty text",
			postMethod: postMethod{
				prefix:      "",
				contentType: "text/plain",
				body:        "",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "Body must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, test.postMethod.prefix}, "/"), strings.NewReader(test.postMethod.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", test.postMethod.contentType)
			request.Header.Set("Accept-Encoding", "")

			resp, err := ts.Client().Do(request)
			require.NoError(t, err)

			// проверяем код ответа
			assert.Equal(t, test.want.code, resp.StatusCode)
			// получаем и проверяем тело запроса
			defer resp.Body.Close()
			resBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if test.want.bodyNotEmpty {
				assert.NotEmpty(t, resBody)
				if len(test.want.preferredBody) != 0 {
					assert.Equal(t, test.want.preferredBody, string(resBody))
				}
			}

			// Проверяем заголовок Content-Type
			assert.Equal(t, test.want.contentType, resp.Header.Get("Content-Type"))
		})
	}
}

func TestGetFullURL(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	ts.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Возвращаем ошибку, чтобы остановить автоматическое следование
		return http.ErrUseLastResponse
	}

	t.Run("positive test", func(t *testing.T) {
		body := "https://practicum.yandex.ru"
		postRequest, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(body))
		require.NoError(t, err)
		postRequest.Header.Set("Content-Type", "text/plain")

		postResp, err := ts.Client().Do(postRequest)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, postResp.StatusCode)

		postBuf, err := io.ReadAll(postResp.Body)
		require.NoError(t, err)
		defer postResp.Body.Close()

		getRequest, err := http.NewRequest(http.MethodGet, string(postBuf), nil)
		require.NoError(t, err)

		getResp, err := ts.Client().Do(getRequest)
		require.NoError(t, err)
		defer getResp.Body.Close()

		locationHeader := getResp.Header.Get("Location")

		assert.Equal(t, http.StatusTemporaryRedirect, getResp.StatusCode)
		assert.Equal(t, body, locationHeader)
	})

	t.Run("can't find fullURL", func(t *testing.T) {
		const shortURL = "ASDQWE"
		reqURL, err := url.JoinPath(ts.URL, "/", shortURL)
		require.NoError(t, err)
		getRequest, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)
		getRequest.Header.Set("Accept-Encoding", "")

		getResp, err := ts.Client().Do(getRequest)
		require.NoError(t, err)
		defer getResp.Body.Close()

		getBuf, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, getResp.StatusCode)
		assert.Equal(t, "Invalid URL in request\n", string(getBuf))
	})
}

func TestPostConflictFullURL(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	body := "https://abc.net"

	request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, "/"}, ""), strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("Accept-Encoding", "")

	resp, err := ts.Client().Do(request)
	require.NoError(t, err)

	// проверяем код ответа
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	defer resp.Body.Close()

	repeatReq, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, "/"}, ""), strings.NewReader(body))
	require.NoError(t, err)
	repeatReq.Header.Set("Content-Type", "text/plain")
	repeatReq.Header.Set("Accept-Encoding", "")

	repeatResp, err := ts.Client().Do(repeatReq)
	require.NoError(t, err)

	// проверяем код ответа
	assert.Equal(t, http.StatusConflict, repeatResp.StatusCode)
	defer repeatResp.Body.Close()
}

// readAuditMessages ждёт, пока в аудит-файле появится ровно want записей, и возвращает их.
func readAuditMessages(t *testing.T, path string, want int) []model.AuditServiceMessage {
	t.Helper()

	var messages []model.AuditServiceMessage
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}

		messages = nil
		dec := json.NewDecoder(bytes.NewReader(data))
		for dec.More() {
			var m model.AuditServiceMessage
			if err := dec.Decode(&m); err != nil {
				return false
			}
			messages = append(messages, m)
		}
		return len(messages) == want
	}, 2*time.Second, 10*time.Millisecond)

	return messages
}

func TestAuditPostAndFollow(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	ts := setUpServerWithAudit(t, config.AuditServiceConfig{FilePath: auditPath})
	ts.Start()
	defer ts.Close()

	ts.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	const originalURL = "https://audit.example.com"

	postReq, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(originalURL))
	require.NoError(t, err)
	postReq.Header.Set("Content-Type", "text/plain")
	postReq.Header.Set("Accept-Encoding", "")

	postResp, err := ts.Client().Do(postReq)
	require.NoError(t, err)
	defer postResp.Body.Close()
	require.Equal(t, http.StatusCreated, postResp.StatusCode)

	shortURL, err := io.ReadAll(postResp.Body)
	require.NoError(t, err)

	messages := readAuditMessages(t, auditPath, 1)
	assert.Equal(t, "shorten", messages[0].Action)
	assert.Equal(t, originalURL, messages[0].OriginalURL)
	assert.NotEmpty(t, messages[0].Timestamp)

	conflictReq, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(originalURL))
	require.NoError(t, err)
	conflictReq.Header.Set("Content-Type", "text/plain")
	conflictReq.Header.Set("Accept-Encoding", "")

	conflictResp, err := ts.Client().Do(conflictReq)
	require.NoError(t, err)
	defer conflictResp.Body.Close()
	require.Equal(t, http.StatusConflict, conflictResp.StatusCode)

	getReq, err := http.NewRequest(http.MethodGet, string(shortURL), nil)
	require.NoError(t, err)

	getResp, err := ts.Client().Do(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusTemporaryRedirect, getResp.StatusCode)

	messages = readAuditMessages(t, auditPath, 2)
	assert.Equal(t, "shorten", messages[0].Action)
	assert.Equal(t, "follow", messages[1].Action)
	assert.Equal(t, originalURL, messages[1].OriginalURL)
	assert.NotEmpty(t, messages[1].Timestamp)
}
