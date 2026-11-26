DROP TRIGGER IF EXISTS trg_post_access_post_validate ON post;
DROP TRIGGER IF EXISTS trg_post_access_stage_validate ON post_access_stage;
DROP FUNCTION IF EXISTS validate_post_access_stage_before_end();

DROP TABLE IF EXISTS post_access_stage;
ALTER TABLE post
    DROP COLUMN publication_ends_at;
