ALTER TABLE shortener_urls
DROP COLUMN IF EXISTS is_deleted;

DROP INDEX IF EXISTS idx_user_id;