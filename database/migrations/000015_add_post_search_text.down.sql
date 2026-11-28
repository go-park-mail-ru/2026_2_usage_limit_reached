DROP INDEX IF EXISTS post_search_text_trgm_idx;
DROP INDEX IF EXISTS post_title_trgm_idx;

ALTER TABLE post DROP COLUMN IF EXISTS search_text;
