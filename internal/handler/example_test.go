package handler_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"

	"github.com/apnaumov/url-shortener.git/internal/config"
	"github.com/apnaumov/url-shortener.git/internal/handler"
	"github.com/apnaumov/url-shortener.git/internal/logger"
	"github.com/apnaumov/url-shortener.git/internal/repository"
)

// newExampleServer поднимает тестовый сервер с in-memory хранилищем
// и возвращает его вместе с клиентом, который хранит cookie (аутентификация)
// и не следует редиректам.
func newExampleServer() (*httptest.Server, *http.Client, error) {
	ts := httptest.NewUnstartedServer(nil)

	storageConf := config.PendingMessageProcessorConfig{
		TickTime:                  10,
		BufferSize:                1024,
		MaxBatchSize:              512,
		WorkerPoolSize:            1,
		MaxParallelInsertsToQueue: 20,
	}

	l, err := logger.InitializeRootLogger("example", "error")
	if err != nil {
		return nil, nil, err
	}

	storage, err := repository.NewRuntimeStorage("", &storageConf, l)
	if err != nil {
		return nil, nil, err
	}

	router, err := handler.NewURLShortenerRouter(
		"http://"+ts.Listener.Addr().String(), []byte("example-secret-key"), storage, config.AuditServiceConfig{})
	if err != nil {
		return nil, nil, err
	}

	ts.Config.Handler = router.Mux
	ts.Start()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, err
	}

	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return ts, client, nil
}

// Пример сокращения URL через POST / с телом text/plain.
// Повторный запрос с тем же URL возвращает 409 и уже существующую короткую ссылку.
func Example_postURL() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	for range 2 {
		resp, err := client.Post(ts.URL+"/", "text/plain", strings.NewReader("https://practicum.yandex.ru/"))
		if err != nil {
			fmt.Println(err)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Println(resp.StatusCode, strings.HasPrefix(string(body), ts.URL+"/"))
	}

	// Output:
	// 201 true
	// 409 true
}

// Пример редиректа по короткой ссылке: GET /{shortPath} отвечает 307 и заголовком Location.
func Example_getFullURL() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	resp, err := client.Post(ts.URL+"/", "text/plain", strings.NewReader("https://practicum.yandex.ru/"))
	if err != nil {
		fmt.Println(err)
		return
	}
	shortURL, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	resp, err = client.Get(string(shortURL))
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	fmt.Println(resp.StatusCode, resp.Header.Get("Location"))

	// Output:
	// 307 https://practicum.yandex.ru/
}

// Пример сокращения URL через JSON API: POST /api/shorten.
func Example_apiShorten() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	resp, err := client.Post(ts.URL+"/api/shorten", "application/json",
		strings.NewReader(`{"URL": "https://practicum.yandex.ru/"}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode, strings.HasPrefix(result.Result, ts.URL+"/"))

	// Output:
	// 201 true
}

// Пример пакетного сокращения: POST /api/shorten/batch.
// Каждому элементу ответа соответствует correlation_id из запроса.
func Example_apiShortenBatch() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	reqBody := `[
		{"correlation_id": "1", "original_URL": "https://practicum.yandex.ru/"},
		{"correlation_id": "2", "original_URL": "https://go.dev/"}
	]`

	resp, err := client.Post(ts.URL+"/api/shorten/batch", "application/json", strings.NewReader(reqBody))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	var result []struct {
		ShortURL      string `json:"short_URL"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.StatusCode, len(result))

	// Output:
	// 201 2
}

// Пример работы со ссылками пользователя: GET /api/user/urls возвращает список
// (204, если он пуст), DELETE /api/user/urls принимает короткие ключи и удаляет их асинхронно (202).
// Пользователь определяется по cookie, которую сервер выдаёт при первом запросе.
func Example_apiUserURLs() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	// Создаём ссылку.
	resp, err := client.Post(ts.URL+"/api/shorten", "application/json",
		strings.NewReader(`{"URL": "https://practicum.yandex.ru/"}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	// Теперь список не пуст.
	resp, err = client.Get(ts.URL + "/api/user/urls")
	if err != nil {
		fmt.Println(err)
		return
	}

	var urls []struct {
		ShortURL    string `json:"short_URL"`
		OriginalURL string `json:"original_URL"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&urls); err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()
	fmt.Println(resp.StatusCode, urls[0].OriginalURL)

	// Удаляем ссылку по короткому ключу.
	key := strings.TrimPrefix(urls[0].ShortURL, ts.URL+"/")
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", strings.NewReader(`["`+key+`"]`))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()
	fmt.Println(resp.StatusCode)

	// Output:
	// 200 https://practicum.yandex.ru/
	// 202
}

// Пример проверки доступности хранилища: GET /ping.
func Example_ping() {
	ts, client, err := newExampleServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ts.Close()

	resp, err := client.Get(ts.URL + "/ping")
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Body.Close()

	fmt.Println(resp.StatusCode)

	// Output:
	// 200
}
