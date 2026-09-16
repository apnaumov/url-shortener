package repository

const (
	getFullUrlQuery   = "SELECT full_url FROM shortener_urls WHERE short_url = $1"
	getUserUrls       = "SELECT short_url, full_url FROM shortener_urls WHERE user_id = $1"
	checkCollisionUrl = "SELECT 1 FROM shortener_urls WHERE short_url = $1"
	setFullUrlQuery   = `INSERT INTO shortener_urls (short_url, full_url, correlation_id, user_id) 
							VALUES ($1, $2, $3, $4) 
						ON CONFLICT (full_url) DO 
							UPDATE SET full_url = shortener_urls.full_url RETURNING short_url, correlation_id`
	insertNewUserId = "INSERT INTO users DEFAULT VALUES RETURNING user_id;"
)
