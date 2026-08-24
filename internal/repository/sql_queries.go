package repository

const (
	getFullUrlQuery           = "SELECT fullUrl FROM shortener_urls WHERE shortUrl = $1"
	checkShortUrlIsExistQuery = "SELECT 1 FROM shortener_urls WHERE shortUrl = $1"
	setFullUrlQuery           = "INSERT INTO shortener_urls (shortUrl, fullUrl) VALUES ($1, $2)"
)
