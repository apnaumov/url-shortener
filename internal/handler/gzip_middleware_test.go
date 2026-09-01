package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipCompression(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	successBodyRegExp := `^\{.*"result":\s*".*".*\}`
	contentType := "application/json"

	const postPrefix = "/api/shorten"

	t.Run("sends_gzip", func(t *testing.T) {
		requestBody := `{ "url": "https://abc.net"}`
		buf := bytes.NewBuffer(nil)
		zb := gzip.NewWriter(buf)
		_, err := zb.Write([]byte(requestBody))
		require.NoError(t, err)
		err = zb.Close()
		require.NoError(t, err)

		request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, postPrefix}, ""), buf)
		require.NoError(t, err)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Content-Encoding", "gzip")
		request.Header.Set("Accept-Encoding", "")

		resp, err := ts.Client().Do(request)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		reg, err := regexp.Compile(successBodyRegExp)
		require.NoError(t, err)
		assert.True(t, reg.MatchString(string(b)))
	})

	t.Run("accepts_gzip", func(t *testing.T) {
		requestBody := `{ "url": "https://abcd.net"}`
		request, err := http.NewRequest(http.MethodPost, strings.Join([]string{ts.URL, postPrefix}, ""), strings.NewReader(requestBody))
		require.NoError(t, err)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Accept-Encoding", "gzip")

		resp, err := ts.Client().Do(request)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		defer resp.Body.Close()

		zr, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)

		b, err := io.ReadAll(zr)
		require.NoError(t, err)

		reg, err := regexp.Compile(successBodyRegExp)
		require.NoError(t, err)
		assert.True(t, reg.MatchString(string(b)))
	})
}
