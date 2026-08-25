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
	const postPrefix = "/api/shorten"

	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	type postMethod struct {
		contentType string
		body        string
	}

	type want struct {
		code            int
		bodyNotEmpty    bool
		prefferedBody   string
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
				contentType: "application/json",
				body:        `{ "url": "https://abc.net"}`,
			},
			want: want{
				code:            http.StatusCreated,
				bodyNotEmpty:    true,
				contentType:     "application/json",
				wantMatchRegExp: true,
				prefferedBody:   `^\{.*"result":\s*".*".*\}`,
			},
		},
		{
			name: "wrong content type test",
			postMethod: postMethod{
				body:        "test text plain",
				contentType: "text/plain",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				prefferedBody: "Content-type incorrect\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "body must be not empty text",
			postMethod: postMethod{
				contentType: "application/json",
				body:        "",
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				prefferedBody: "Body must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "wrong json",
			postMethod: postMethod{
				contentType: "application/json",
				body:        "a",
			},
			want: want{
				code:          500,
				bodyNotEmpty:  true,
				prefferedBody: "Internal Server Error\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
		{
			name: "empty URL",
			postMethod: postMethod{
				contentType: "application/json",
				body:        `{ "url": ""}`,
			},
			want: want{
				code:          400,
				bodyNotEmpty:  true,
				prefferedBody: "URL must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, postPrefix}, ""), strings.NewReader(test.postMethod.body))
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
				if len(test.want.prefferedBody) != 0 {
					if test.want.wantMatchRegExp {
						reg, err := regexp.Compile(test.want.prefferedBody)
						require.NoError(t, err)
						assert.True(t, reg.MatchString(string(resBody)))
					} else {
						assert.Equal(t, test.want.prefferedBody, string(resBody))
					}
				}
			}

			// Проверяем заголовок Content-Type
			assert.Equal(t, test.want.contentType, resp.Header.Get("Content-Type"))
		})
	}
}
