-- Поисковый текст поста: видимый текст body без разметки.
-- Считается приложением тем же санитайзером, что и body, поэтому триггер не нужен.
-- Существующие посты пересчитываются командой cmd/reindex-posts после деплоя

ALTER TABLE post ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX post_search_text_trgm_idx ON post USING gin (search_text gin_trgm_ops);
CREATE INDEX post_title_trgm_idx ON post USING gin (title gin_trgm_ops);
