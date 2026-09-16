CREATE TABLE IF NOT EXISTS shortener_url_users (
	user_id SERIAL NOT NULL PRIMARY KEY
);

ALTER TABLE shortener_urls
ADD COLUMN IF NOT EXISTS user_id INTEGER;

ALTER TABLE shortener_urls
ADD CONSTRAINT fk_shortener_urls_user
FOREIGN KEY (user_id) REFERENCES shortener_url_users(user_id)
ON DELETE CASCADE 
ON UPDATE CASCADE;
