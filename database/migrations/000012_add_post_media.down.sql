DROP TABLE IF EXISTS post_block CASCADE;
DROP TABLE IF EXISTS media CASCADE;

CREATE TABLE post_block (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id      UUID NOT NULL,
    position     INTEGER NOT NULL,
    media_type   TEXT NOT NULL,
    media_key    TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_post_block_key UNIQUE (media_key),
    CONSTRAINT uk_post_block_position UNIQUE (post_id, position),
    CONSTRAINT fk_post_block_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT chk_post_block_size
        CHECK (size_bytes > 0),
    CONSTRAINT chk_post_block_position
        CHECK (position >= 0),
    CONSTRAINT chk_post_block_media_type
        CHECK (media_type IN ('image', 'video', 'audio', 'file'))
);

CREATE TRIGGER trg_post_block_set_updated_at
    BEFORE UPDATE ON post_block
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
