ALTER TABLE shortener_urls DROP CONSTRAINT IF EXISTS fk_shortener_urls_user;
ALTER TABLE shortener_urls DROP COLUMN IF EXISTS user_id;
DROP TABLE IF EXISTS users; 