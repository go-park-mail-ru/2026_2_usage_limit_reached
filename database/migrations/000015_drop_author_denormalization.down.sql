DROP INDEX IF EXISTS user_username_trgm_idx;
DROP INDEX IF EXISTS user_nickname_trgm_idx;
DROP INDEX IF EXISTS user_nickname_order_idx;

ALTER TABLE author
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS nickname,
    DROP COLUMN IF EXISTS is_active;
