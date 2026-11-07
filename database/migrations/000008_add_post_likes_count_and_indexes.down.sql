DROP INDEX IF EXISTS idx_post_popular_feed;

DROP TRIGGER IF EXISTS trg_post_like_delete ON post_like;
DROP TRIGGER IF EXISTS trg_post_like_insert ON post_like;

DROP FUNCTION IF EXISTS update_post_likes_count();

DROP INDEX IF EXISTS post_like_post_id_user_id_idx;