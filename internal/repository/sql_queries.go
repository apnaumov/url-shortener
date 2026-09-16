package repository

const (
	getFullURLQuery   = "SELECT full_URL FROM shortener_URLs WHERE short_URL = $1"
	getUserURLs       = "SELECT short_URL, full_URL FROM shortener_URLs WHERE user_id = $1"
	checkCollisionURL = "SELECT 1 FROM shortener_URLs WHERE short_URL = $1"
	setFullURLQuery   = `INSERT INTO shortener_URLs (short_URL, full_URL, correlation_id, user_id) 
							VALUES ($1, $2, $3, $4) 
						ON CONFLICT (full_URL) DO 
							UPDATE SET full_URL = shortener_URLs.full_URL RETURNING short_URL, correlation_id`
	insertNewuserID = "INSERT INTO users DEFAULT VALUES RETURNING user_id;"
)
