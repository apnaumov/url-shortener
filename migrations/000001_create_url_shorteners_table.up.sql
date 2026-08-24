CREATE TABLE shortener_urls (
	internal_id SERIAL NOT NULL PRIMARY KEY,
	shortUrl TEXT NOT NULL,
	fullUrl TEXT NOT NULL
);

-- Базовый индекс для поиска по короткому URL
CREATE INDEX idx_short_url ON shortener_urls(shortUrl);