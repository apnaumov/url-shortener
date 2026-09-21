ALTER TABLE shortener_urls
ADD COLUMN IF NOT EXISTS is_deleted boolean NOT NULL DEFAULT false;