CREATE TABLE IF NOT EXISTS shortener_urls (
	internal_id SERIAL NOT NULL PRIMARY KEY,
	short_url TEXT NOT NULL,
	full_url TEXT NOT NULL
);

-- Базовый индекс для поиска по короткому URL
CREATE INDEX IF NOT EXISTS idx_short_url ON shortener_urls(short_url);