package model

type PostURL struct {
	URL string `json:"url"`
}

type ResultShortenURL struct {
	Result string `json:"result"`
}

type URLFileRecord struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}
