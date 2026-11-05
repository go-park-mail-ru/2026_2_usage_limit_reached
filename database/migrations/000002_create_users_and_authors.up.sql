-- Таблица пользователей платформы
CREATE TABLE "user" (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username       TEXT NOT NULL,
    nickname       TEXT NOT NULL,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    avatar_key     TEXT,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_user_username UNIQUE (username),
    CONSTRAINT uk_user_email UNIQUE (email),
    CONSTRAINT chk_user_status
        CHECK (status IN ('active', 'blocked', 'deleted'))
);

CREATE TRIGGER trg_user_set_updated_at
    BEFORE UPDATE ON "user"
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Профиль автора (1-к-1 с user)
CREATE TABLE author (
    id                 UUID PRIMARY KEY,
    bio                TEXT NOT NULL DEFAULT '',
    category           TEXT,
    payout_provider    TEXT,
    payout_account_id  TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_author_user
        FOREIGN KEY (id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT uk_author_payout_provider_payout_account_id
        UNIQUE (payout_provider, payout_account_id),
    CONSTRAINT chk_author_payout_pair
        CHECK ((payout_provider IS NULL) = (payout_account_id IS NULL))
);

CREATE TRIGGER trg_author_set_updated_at
    BEFORE UPDATE ON author
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();