package model

type PostURL struct {
	URL string `json:"url"`
}

type ResultShortenURL struct {
	Result string `json:"result"`
}

type URLRecord struct {
	ShortURL string `json:"short_url"`
	UrlData  RequestURLData
}

type RequestURLData struct {
	OriginalURL   string `json:"original_url"`
	CorrelationId string `json:"correlation_id"`
}

type ResponceURLData struct {
	ShortUrl      string `json:"short_url"`
	CorrelationId string `json:"correlation_id"`
}
