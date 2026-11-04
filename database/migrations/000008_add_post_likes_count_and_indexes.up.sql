ALTER TABLE post ADD COLUMN likes_count BIGINT DEFAULT 0;
CREATE INDEX post_like_post_id_user_id_idx ON post_like (post_id, user_id);

CREATE OR REPLACE FUNCTION update_post_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE post SET likes_count = likes_count + 1 WHERE id = NEW.post_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE post SET likes_count = likes_count - 1 WHERE id = OLD.post_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_like_insert
AFTER INSERT ON post_like
FOR EACH ROW
EXECUTE FUNCTION update_post_likes_count();

CREATE TRIGGER trg_post_like_delete
AFTER DELETE ON post_like
FOR EACH ROW
EXECUTE FUNCTION update_post_likes_count();

CREATE INDEX idx_post_popular_feed
ON post (likes_count DESC, published_at DESC, id DESC)
WHERE status = 'published';