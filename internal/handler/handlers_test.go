package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootMiddleware(t *testing.T) {
	t.Run("Wrond method", func(t *testing.T) {
		body := "https://practicum.yandex.ru"
		putRequest := httptest.NewRequest(http.MethodPut, serverAddr+"/", strings.NewReader(body))

		putW := httptest.NewRecorder()

		RootMiddleware("http://localhost:8080").ServeHTTP(putW, putRequest)

		putBuf, putReadErr := io.ReadAll(putW.Result().Body)
		assert.Nil(t, putReadErr)
		putW.Result().Body.Close()

		assert.Equal(t, "Method not allowed\n", string(putBuf))
		assert.Equal(t, http.StatusBadRequest, putW.Result().StatusCode)

	})
}

func TestPostNewURL(t *testing.T) {
	serverAddr = "http://localhost:8080"

	type postMethod struct {
		prefix      string
		contentType string
		body        string
	}

	type wantError struct {
		hasError bool
		errorRes string
	}

	type want struct {
		code         int
		bodyNotEmpty bool
		contentType  string
		wantError    wantError
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
				code:         201,
				bodyNotEmpty: true,
				contentType:  "text/plain",
				wantError:    wantError{hasError: false},
			},
		},
		{
			name: "method not allowed test",
			postMethod: postMethod{
				prefix:      "try/to/check/another/prefix/",
				contentType: "text/plain",
				body:        "body",
			},
			want: want{
				wantError: wantError{hasError: true, errorRes: "Method not allowed"},
			},
		},
		{
			name: "wrong content type test",
			postMethod: postMethod{
				prefix:      "",
				contentType: "application/json",
			},
			want: want{
				wantError: wantError{hasError: true, errorRes: "Content-type incorrect"},
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
				wantError: wantError{hasError: true, errorRes: "Body must be not empty"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, strings.Join([]string{serverAddr, test.postMethod.prefix}, "/"), strings.NewReader(test.postMethod.body))
			request.Header.Set("Content-Type", test.postMethod.contentType)

			// создаём новый Recorder
			w := httptest.NewRecorder()

			err := postNewURL(w, request)

			if test.want.wantError.hasError {
				assert.NotNil(t, err)
				assert.Equal(t, test.want.wantError.errorRes, err.Error())
				return
			}

			res := w.Result()

			// проверяем код ответа
			assert.Equal(t, test.want.code, res.StatusCode)
			// получаем и проверяем тело запроса
			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			if test.want.bodyNotEmpty {
				assert.NotEmpty(t, resBody)
			}

			// Проверяем заголовок Content-Type
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))
		})
	}
}

func TestGetFullUrl(t *testing.T) {
	serverAddr = "http://localhost:8080"

	t.Run("positive test", func(t *testing.T) {
		body := "https://practicum.yandex.ru"
		postRequest := httptest.NewRequest(http.MethodPost, serverAddr+"/", strings.NewReader(body))
		postRequest.Header.Set("Content-Type", "text/plain")

		postW := httptest.NewRecorder()

		postErr := postNewURL(postW, postRequest)

		assert.Nil(t, postErr)

		postBuf, postReadErr := io.ReadAll(postW.Result().Body)
		assert.Nil(t, postReadErr)
		postW.Result().Body.Close()

		getRequest := httptest.NewRequest(http.MethodGet, string(postBuf), nil)

		getW := httptest.NewRecorder()

		getErr := getFullURL(getW, getRequest)
		assert.Nil(t, getErr)

		locationHeader := getW.Result().Header.Get("Location")

		assert.Equal(t, getW.Result().StatusCode, http.StatusTemporaryRedirect)
		assert.Equal(t, body, locationHeader)
	})

	t.Run("method not allowed", func(t *testing.T) {
		getRequest := httptest.NewRequest(http.MethodGet, serverAddr+"/", nil)

		getW := httptest.NewRecorder()

		getErr := getFullURL(getW, getRequest)

		assert.NotNil(t, getErr)
		assert.Equal(t, "Method not allowed", getErr.Error())
	})

	t.Run("can't find fullUrl", func(t *testing.T) {
		getRequest := httptest.NewRequest(http.MethodGet, serverAddr+"/ASDQWE", nil)

		getW := httptest.NewRecorder()

		getErr := getFullURL(getW, getRequest)

		assert.NotNil(t, getErr)
		assert.Equal(t, "Can't find URL", getErr.Error())
	})
}
