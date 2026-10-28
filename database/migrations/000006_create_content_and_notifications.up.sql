-- Публикации авторов
CREATE TABLE post (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id     UUID NOT NULL,
    title         TEXT NOT NULL,
    body          TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'draft',
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_post_author
        FOREIGN KEY (author_id)
        REFERENCES author (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_post_status
        CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT chk_post_published_requirements
        CHECK (
            status <> 'published'
            OR (published_at IS NOT NULL AND length(trim(title)) > 0)
        )
);

CREATE TRIGGER trg_post_set_updated_at
    BEFORE UPDATE ON post
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Доступ уровней подписки к посту
-- Отсутствие строки = пост общедоступен
CREATE TABLE post_access (
    post_id                UUID PRIMARY KEY,
    subscription_type_id   UUID NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_post_access_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_post_access_type
        FOREIGN KEY (subscription_type_id)
        REFERENCES subscription_type (id)
        ON DELETE RESTRICT
);

CREATE TRIGGER trg_post_access_set_updated_at
    BEFORE UPDATE ON post_access
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Медиа-блоки внутри поста
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

-- Лайки постов
CREATE TABLE post_like (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID NOT NULL,
    user_id     UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_post_like_user_post UNIQUE (post_id, user_id),
    CONSTRAINT fk_post_like_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_post_like_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE CASCADE
);

CREATE TRIGGER trg_post_like_set_updated_at
    BEFORE UPDATE ON post_like
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Комментарии к посту
CREATE TABLE comment (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID NOT NULL,
    user_id     UUID NOT NULL,
    text        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_comment_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comment_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_comment_text_not_blank
        CHECK (length(trim(text)) > 0)
);

CREATE INDEX comment_post_id_idx ON "comment" (post_id);
CREATE INDEX comment_user_id_idx ON "comment" (user_id);

CREATE TRIGGER trg_comment_set_updated_at
    BEFORE UPDATE ON comment
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Лайки комментариев
CREATE TABLE comment_like (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id  UUID NOT NULL,
    user_id     UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_comment_like_user UNIQUE (comment_id, user_id),
    CONSTRAINT fk_comment_like_comment
        FOREIGN KEY (comment_id)
        REFERENCES comment (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comment_like_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE CASCADE
);

CREATE INDEX comment_like_user_id_idx ON comment_like (user_id);

CREATE TRIGGER trg_comment_like_set_updated_at
    BEFORE UPDATE ON comment_like
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Уведомления пользователям
CREATE TABLE notification (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL,
    event_kind        TEXT NOT NULL,
    post_id           UUID,
    comment_id        UUID,
    comment_like_id   UUID,
    post_like_id      UUID,
    subscription_id   UUID,
    donation_id       UUID,
    scheduled_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at           TIMESTAMPTZ,
    read_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_notification_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_notification_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notification_comment
        FOREIGN KEY (comment_id)
        REFERENCES comment (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notification_comment_like
        FOREIGN KEY (comment_like_id)
        REFERENCES comment_like (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notification_post_like
        FOREIGN KEY (post_like_id)
        REFERENCES post_like (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notification_sub
        FOREIGN KEY (subscription_id)
        REFERENCES subscription (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notification_donation
        FOREIGN KEY (donation_id)
        REFERENCES donation (payment_order_id)
        ON DELETE CASCADE,
    CONSTRAINT chk_notification_event_kind
        CHECK (event_kind IN (
            'post_published', 'comment_created', 'comment_liked',
            'post_liked', 'subscription_changed', 'donation_received'
        )),
    CONSTRAINT chk_notification_source_kind
        CHECK (
            num_nonnulls(
                post_id, comment_id, comment_like_id,
                post_like_id, subscription_id, donation_id
            ) = 1
            AND (
                (event_kind = 'post_published' AND post_id IS NOT NULL)
                OR (event_kind = 'comment_created' AND comment_id IS NOT NULL)
                OR (event_kind = 'comment_liked' AND comment_like_id IS NOT NULL)
                OR (event_kind = 'post_liked' AND post_like_id IS NOT NULL)
                OR (event_kind = 'subscription_changed' AND subscription_id IS NOT NULL)
                OR (event_kind = 'donation_received' AND donation_id IS NOT NULL)
            )
        ),
    CONSTRAINT chk_notification_read_after_sent
        CHECK (read_at IS NULL OR (sent_at IS NOT NULL AND read_at >= sent_at))
);

CREATE INDEX notification_user_id_idx ON notification (user_id);
CREATE INDEX notification_post_id_idx ON notification (post_id);
CREATE INDEX notification_comment_id_idx ON notification (comment_id);
CREATE INDEX notification_comment_like_id_idx ON notification (comment_like_id);
CREATE INDEX notification_post_like_id_idx ON notification (post_like_id);
CREATE INDEX notification_subscription_id_idx ON notification (subscription_id);
CREATE INDEX notification_donation_id_idx ON notification (donation_id);

CREATE TRIGGER trg_notification_set_updated_at
    BEFORE UPDATE ON notification
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();