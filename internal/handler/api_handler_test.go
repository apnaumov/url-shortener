package handler

import (
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApiPostNewURL(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	type postMethod struct {
		postPrefix  string
		contentType string
		body        string
	}

	type want struct {
		code            int
		bodyNotEmpty    bool
		preferredBody   string
		wantMatchRegExp bool
		contentType     string
	}
	tests := []struct {
		name       string
		postMethod postMethod
		want       want
	}{
		{
			name: "positive test",
			postMethod: postMethod{
				postPrefix:  "/api/shorten",
				contentType: "application/json",
				body:        `{ "URL": "https://abc.net"}`,
			},
			want: want{
				code:            http.StatusCreated,
				bodyNotEmpty:    true,
				contentType:     "application/json",
				wantMatchRegExp: true,
				preferredBody:   `^\{.*"result":\s*".*".*\}`,
			},
		},
		{
			name: "wrong content type test",
			postMethod: postMethod{
				postPrefix:  "/api/shorten",
				body:        "test text plain",
				contentType: "text/plain",
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
				postPrefix:  "/api/shorten",
				contentType: "application/json",
				body:        "",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "Body must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "wrong json",
			postMethod: postMethod{
				postPrefix:  "/api/shorten",
				contentType: "application/json",
				body:        "a",
			},
			want: want{
				code:          500,
				bodyNotEmpty:  true,
				preferredBody: "Internal Server Error\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "empty URL",
			postMethod: postMethod{
				postPrefix:  "/api/shorten",
				contentType: "application/json",
				body:        `{ "URL": ""}`,
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "URL must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "positive test batch",
			postMethod: postMethod{
				postPrefix:  "/api/shorten/batch",
				contentType: "application/json",
				body:        `[{ "correlation_id": "asd", "original_URL": "def.com"}]`,
			},
			want: want{
				code:         http.StatusCreated,
				bodyNotEmpty: true,
				contentType:  "application/json",
			},
		},
		{
			name: "wrong content type test batch",
			postMethod: postMethod{
				postPrefix:  "/api/shorten/batch",
				body:        "test text plain",
				contentType: "text/plain",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "Content-type incorrect\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "body must be not empty text batch",
			postMethod: postMethod{
				postPrefix:  "/api/shorten/batch",
				contentType: "application/json",
				body:        "",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "Body must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "wrong json batch",
			postMethod: postMethod{
				postPrefix:  "/api/shorten/batch",
				contentType: "application/json",
				body:        "a",
			},
			want: want{
				code:          500,
				bodyNotEmpty:  true,
				preferredBody: "Internal Server Error\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "empty URL batch",
			postMethod: postMethod{
				postPrefix:  "/api/shorten/batch",
				contentType: "application/json",
				body:        `[{ "correlation_id": "", "original_URL": ""}]`,
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				preferredBody: "URL data must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, test.postMethod.postPrefix}, ""), strings.NewReader(test.postMethod.body))
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
					if test.want.wantMatchRegExp {
						reg, err := regexp.Compile(test.want.preferredBody)
						require.NoError(t, err)
						assert.True(t, reg.MatchString(string(resBody)))
					} else {
						assert.Equal(t, test.want.preferredBody, string(resBody))
					}
				}
			}

			// Проверяем заголовок Content-Type
			assert.Equal(t, test.want.contentType, resp.Header.Get("Content-Type"))
		})
	}
}

func TestApiPostConflictFullURL(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	tests := []struct {
		name   string
		prefix string
		body   string
	}{
		{
			name:   "conflict with only URL",
			prefix: "/api/shorten",
			body:   `{ "URL": "https://abc.net"}`,
		},
		{
			name:   "conflict batch URL",
			prefix: "/api/shorten/batch",
			body:   `[{ "correlation_id": "asd", "original_URL": "def.com"}, {"correlation_id": "def", "original_URL": "asd.com"}]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, test.prefix}, ""), strings.NewReader(test.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept-Encoding", "")

			resp, err := ts.Client().Do(request)
			require.NoError(t, err)

			// проверяем код ответа
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
			defer resp.Body.Close()

			repeatReq, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, test.prefix}, ""), strings.NewReader(test.body))
			require.NoError(t, err)
			repeatReq.Header.Set("Content-Type", "application/json")
			repeatReq.Header.Set("Accept-Encoding", "")

			repeatResp, err := ts.Client().Do(repeatReq)
			require.NoError(t, err)

			// проверяем код ответа
			assert.Equal(t, http.StatusConflict, repeatResp.StatusCode)
			defer repeatResp.Body.Close()
		})
	}
}

func TestApiAuditShorten(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	ts := setUpServerWithAudit(t, config.AuditServiceConfig{FilePath: auditPath})
	ts.Start()
	defer ts.Close()

	post := func(t *testing.T, body string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/shorten", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "")

		resp, err := ts.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	require.Equal(t, http.StatusCreated, post(t, `{"URL": "https://api-audit.example.com"}`))

	messages := readAuditMessages(t, auditPath, 1)
	assert.Equal(t, "shorten", messages[0].Action)
	assert.Equal(t, "https://api-audit.example.com", messages[0].OriginalURL)
	assert.NotEmpty(t, messages[0].Timestamp)

	// конфликт и некорректный запрос в аудит не попадают
	require.Equal(t, http.StatusConflict, post(t, `{"URL": "https://api-audit.example.com"}`))
	require.Equal(t, http.StatusBadRequest, post(t, `{"URL": ""}`))

	require.Equal(t, http.StatusCreated, post(t, `{"URL": "https://api-audit2.example.com"}`))
	messages = readAuditMessages(t, auditPath, 2)
	assert.Equal(t, "https://api-audit2.example.com", messages[1].OriginalURL)
}
