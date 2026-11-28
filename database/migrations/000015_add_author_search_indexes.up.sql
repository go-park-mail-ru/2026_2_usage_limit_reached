-- Индексы для поиска авторов (GET /authors?q=...):
--   * trigram-индексы ускоряют ILIKE '%q%' по username и nickname;
--   * (lower(nickname), id) — курсорная сортировка выдачи.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS user_username_trgm_idx ON "user" USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS user_nickname_trgm_idx ON "user" USING gin (nickname gin_trgm_ops);
CREATE INDEX IF NOT EXISTS user_nickname_order_idx ON "user" (lower(nickname), id);
