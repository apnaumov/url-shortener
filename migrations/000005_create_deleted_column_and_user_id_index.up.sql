ALTER TABLE shortener_urls
ADD COLUMN IF NOT EXISTS is_deleted boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_user_id ON shortener_urls(user_id);