package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setUpServer(t *testing.T) *httptest.Server {
	t.Helper()
	// получение URL и последующая настройка
	ts := httptest.NewUnstartedServer(nil)
	router, err := NewUrlShortenerRouter("http://" + ts.Listener.Addr().String())
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
		prefferedBody string
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
				prefferedBody: "Content-type incorrect\n",
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
				prefferedBody: "Body must be not empty\n",
				contentType:   "text/plain; charset=utf-8",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, test.postMethod.prefix}, "/"), strings.NewReader(test.postMethod.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", test.postMethod.contentType)

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
					assert.Equal(t, test.want.prefferedBody, string(resBody))
				}
			}

			// Проверяем заголовок Content-Type
			assert.Equal(t, test.want.contentType, resp.Header.Get("Content-Type"))
		})
	}
}

func TestGetFullUrl(t *testing.T) {
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

	t.Run("can't find fullUrl", func(t *testing.T) {
		const shortURL = "ASDQWE"
		reqURL, err := url.JoinPath(ts.URL, "/", shortURL)
		require.NoError(t, err)
		getRequest, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		getResp, err := ts.Client().Do(getRequest)
		require.NoError(t, err)
		defer getResp.Body.Close()

		getBuf, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, getResp.StatusCode)
		assert.Equal(t, fmt.Sprintf("can't find URL by the key %q\n", shortURL), string(getBuf))
	})
}
