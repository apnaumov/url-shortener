package repository

const (
	getFullURLQuery             = "SELECT full_url, is_deleted FROM shortener_urls WHERE short_url = $1"
	getUserURLs                 = "SELECT short_url, full_url FROM shortener_urls WHERE user_id = $1"
	getFilteredByDeleteUserURLs = getUserURLs + " AND is_deleted = $2"
	checkCollisionURL           = "SELECT 1 FROM shortener_urls WHERE short_url = $1"
	setFullURLQuery             = `INSERT INTO shortener_urls (short_url, full_url, correlation_id, user_id) 
							VALUES ($1, $2, $3, $4) 
						ON CONFLICT (full_url) DO 
							UPDATE SET full_url = shortener_urls.full_url RETURNING short_url, correlation_id`
	insertNewUserID = "SELECT nextval('user_id_seq')"
	// short url уникален, поэтому достаточно по нему искать
	setDeletedURL          = "UPDATE shortener_urls SET is_deleted = true WHERE short_url IN (%s)"
	getURLRecordsForDelete = "SELECT user_id, is_deleted FROM shortener_urls WHERE short_url IN (%s)"
)
