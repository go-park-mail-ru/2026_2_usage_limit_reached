DROP INDEX IF EXISTS author_username_trgm_idx;
DROP INDEX IF EXISTS author_nickname_trgm_idx;
DROP INDEX IF EXISTS author_nickname_order_idx;

ALTER TABLE author
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS nickname,
    DROP COLUMN IF EXISTS is_active;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS user_username_trgm_idx ON "user" USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS user_nickname_trgm_idx ON "user" USING gin (nickname gin_trgm_ops);
CREATE INDEX IF NOT EXISTS user_nickname_order_idx ON "user" (lower(nickname), id);
