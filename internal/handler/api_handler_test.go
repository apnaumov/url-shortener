package handler

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

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
				body:        `{ "url": "https://abc.net"}`,
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
				body:        `{ "url": ""}`,
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
				body:        `[{ "correlation_id": "asd", "original_url": "def.com"}]`,
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
				body:        `[{ "correlation_id": "", "original_url": ""}]`,
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

func TestApiPostConflictFullUrl(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	tests := []struct {
		name   string
		prefix string
		body   string
	}{
		{
			name:   "conflict with only url",
			prefix: "/api/shorten",
			body:   `{ "url": "https://abc.net"}`,
		},
		{
			name:   "conflict batch url",
			prefix: "/api/shorten/batch",
			body:   `[{ "correlation_id": "asd", "original_url": "def.com"}, {"correlation_id": "def", "original_url": "asd.com"}]`,
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
