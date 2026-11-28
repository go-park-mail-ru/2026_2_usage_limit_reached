-- Реестр загруженных медиа. Файл существует до и независимо от поста:
-- сначала загрузка (POST /attachments), затем маркеры в body поста.
CREATE TABLE media (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id   UUID NOT NULL,
    media_type  TEXT NOT NULL,
    mime_type   TEXT NOT NULL,
    media_key   TEXT NOT NULL,
    filename    TEXT NOT NULL,
    size_bytes  BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_media_key UNIQUE (media_key),
    CONSTRAINT fk_media_author
        FOREIGN KEY (author_id)
        REFERENCES author (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_media_size
        CHECK (size_bytes > 0),
    CONSTRAINT chk_media_type
        CHECK (media_type IN ('image', 'video', 'audio', 'file')),
    CONSTRAINT chk_media_mime_type
        CHECK (length(trim(mime_type)) > 0),
    CONSTRAINT chk_media_filename
        CHECK (length(trim(filename)) > 0)
);

CREATE INDEX media_author_id_idx ON media (author_id);

CREATE TRIGGER trg_media_set_updated_at
    BEFORE UPDATE ON media
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- post_block пересобрана: чистая связка «медиа стоит в посте на позиции».
-- Таблица была пустой (никто в неё не писал), пересоздание безопасно.
DROP TABLE IF EXISTS post_block CASCADE;

CREATE TABLE post_block (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID NOT NULL,
    media_id    UUID NOT NULL,
    position    INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_post_block_position UNIQUE (post_id, position),
    CONSTRAINT fk_post_block_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_post_block_media
        FOREIGN KEY (media_id)
        REFERENCES media (id)
        ON DELETE CASCADE,
    CONSTRAINT chk_post_block_position
        CHECK (position >= 0)
);

CREATE INDEX post_block_media_id_idx ON post_block (media_id);

CREATE TRIGGER trg_post_block_set_updated_at
    BEFORE UPDATE ON post_block
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
