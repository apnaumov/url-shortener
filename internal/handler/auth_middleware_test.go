package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth(t *testing.T) {
	ts := setUpServer(t)
	ts.Start()
	defer ts.Close()

	body := "https://practicum.yandex.ru"
	postRequest, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(body))
	require.NoError(t, err)
	postRequest.Header.Set("Content-Type", "text/plain")

	postResp, err := ts.Client().Do(postRequest)
	require.NoError(t, err)

	defer postResp.Body.Close()

	tokenIsFound := false
	cookies := postResp.Cookies()
	// find needed cookie
	for i := range cookies {
		if cookies[i].Name == "shortener_token" {
			tokenIsFound = true
			break
		}
	}

	assert.True(t, tokenIsFound)
}
