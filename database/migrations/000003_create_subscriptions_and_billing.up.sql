-- Уровни платной подписки автора
CREATE TABLE subscription_type (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    UUID NOT NULL,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL,
    level        SMALLINT NOT NULL,
    retired_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_subscription_type_author
        FOREIGN KEY (author_id)
        REFERENCES author (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_subscription_type_level
        CHECK (level >= 1)
);

-- Уникальны только среди неархивных типов -- после retired_at имя и уровень
-- можно переиспользовать для нового типа того же автора
CREATE UNIQUE INDEX uk_subscription_type_active_name
    ON subscription_type (author_id, name)
    WHERE retired_at IS NULL;

CREATE UNIQUE INDEX uk_subscription_type_active_level
    ON subscription_type (author_id, level)
    WHERE retired_at IS NULL;

CREATE INDEX subscription_type_author_id_idx ON subscription_type (author_id);

CREATE TRIGGER trg_subscription_type_immutable
    BEFORE UPDATE ON subscription_type
    FOR EACH ROW
    WHEN (
        OLD.author_id IS DISTINCT FROM NEW.author_id
        OR OLD.level IS DISTINCT FROM NEW.level
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_subscription_type_set_updated_at
    BEFORE UPDATE ON subscription_type
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Тарифные планы
CREATE TABLE billing_plan (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_type_id   UUID NOT NULL,
    interval_unit          TEXT NOT NULL,
    interval_count         SMALLINT NOT NULL,
    amount_minor           BIGINT NOT NULL,
    currency               TEXT NOT NULL,
    retired_at             TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_billing_plan_type
        FOREIGN KEY (subscription_type_id)
        REFERENCES subscription_type (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_billing_plan_interval
        CHECK (interval_count > 0),
    CONSTRAINT chk_billing_plan_amount
        CHECK (amount_minor > 0),
    CONSTRAINT chk_billing_plan_currency
        CHECK (length(currency) = 3),
    CONSTRAINT chk_billing_plan_interval_unit
        CHECK (interval_unit IN ('day', 'month', 'year'))
);

CREATE INDEX billing_plan_subscription_type_id_idx ON billing_plan (subscription_type_id);

-- Для действующих планов набор коммерческих условий уникален
CREATE UNIQUE INDEX uk_billing_plan_active_terms
    ON billing_plan (subscription_type_id, interval_unit, interval_count, currency)
    WHERE retired_at IS NULL;

CREATE TRIGGER trg_billing_plan_immutable
    BEFORE UPDATE ON billing_plan
    FOR EACH ROW
    WHEN (
        OLD.subscription_type_id IS DISTINCT FROM NEW.subscription_type_id
        OR OLD.interval_unit IS DISTINCT FROM NEW.interval_unit
        OR OLD.interval_count IS DISTINCT FROM NEW.interval_count
        OR OLD.amount_minor IS DISTINCT FROM NEW.amount_minor
        OR OLD.currency IS DISTINCT FROM NEW.currency
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_billing_plan_set_updated_at
    BEFORE UPDATE ON billing_plan
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Договор пользователя с автором
CREATE TABLE subscription (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                     UUID NOT NULL,
    billing_plan_id             UUID NOT NULL,
    status                      TEXT NOT NULL DEFAULT 'pending',
    started_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancellation_requested_at   TIMESTAMPTZ,
    ended_at                    TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_subscription_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_subscription_plan
        FOREIGN KEY (billing_plan_id)
        REFERENCES billing_plan (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_subscription_status
        CHECK (status IN ('pending', 'active', 'paused', 'canceled', 'expired')),
    CONSTRAINT chk_subscription_ended_at_matches_status
        CHECK ((status IN ('canceled', 'expired')) = (ended_at IS NOT NULL)),
    CONSTRAINT chk_subscription_ended_after_started
        CHECK (ended_at IS NULL OR ended_at >= started_at),
    CONSTRAINT chk_subscription_expired_no_cancellation
        CHECK (status <> 'expired' OR cancellation_requested_at IS NULL)
);

CREATE INDEX subscription_user_id_idx ON subscription (user_id);
CREATE INDEX subscription_billing_plan_id_idx ON subscription (billing_plan_id);

CREATE TRIGGER trg_subscription_immutable
    BEFORE UPDATE ON subscription
    FOR EACH ROW
    WHEN (
        OLD.user_id IS DISTINCT FROM NEW.user_id
        OR OLD.started_at IS DISTINCT FROM NEW.started_at
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_subscription_no_delete
    BEFORE DELETE ON subscription
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_subscription_set_updated_at
    BEFORE UPDATE ON subscription
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Запрет самоподписки
CREATE OR REPLACE FUNCTION check_subscription_not_self()
RETURNS TRIGGER AS $$
DECLARE
    v_author_id UUID;
BEGIN
    SELECT st.author_id INTO v_author_id
      FROM billing_plan bp
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE bp.id = NEW.billing_plan_id;

    IF v_author_id = NEW.user_id THEN
        RAISE EXCEPTION 'user % cannot subscribe to their own author profile',
            NEW.user_id
            USING ERRCODE = 'check_violation',
                  CONSTRAINT = 'subscription_not_self';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_not_self
    BEFORE INSERT OR UPDATE OF billing_plan_id ON subscription
    FOR EACH ROW
    EXECUTE FUNCTION check_subscription_not_self();

-- Смена billing_plan_id договора допускается только на план того же автора
CREATE OR REPLACE FUNCTION check_subscription_plan_change_same_author()
RETURNS TRIGGER AS $$
DECLARE
    v_old_author_id UUID;
    v_new_author_id UUID;
BEGIN
    IF OLD.billing_plan_id = NEW.billing_plan_id THEN
        RETURN NEW;
    END IF;

    SELECT st.author_id INTO v_old_author_id
      FROM billing_plan bp
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE bp.id = OLD.billing_plan_id;

    SELECT st.author_id INTO v_new_author_id
      FROM billing_plan bp
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE bp.id = NEW.billing_plan_id;

    IF v_new_author_id IS DISTINCT FROM v_old_author_id THEN
        RAISE EXCEPTION
            'subscription %: changing billing_plan to a different author is not allowed',
            NEW.id
            USING ERRCODE = 'check_violation',
                  CONSTRAINT = 'subscription_plan_change_same_author';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_plan_change_same_author
    BEFORE UPDATE OF billing_plan_id ON subscription
    FOR EACH ROW
    EXECUTE FUNCTION check_subscription_plan_change_same_author();

-- Не более одного незавершенного договора на пару пользователь-автор
CREATE OR REPLACE FUNCTION enforce_unique_non_terminal_subscription()
RETURNS TRIGGER AS $$
DECLARE
    v_author_id       UUID;
    v_conflict_count  INTEGER;
BEGIN
    IF NEW.status NOT IN ('pending', 'active', 'paused') THEN
        RETURN NEW;
    END IF;

    SELECT st.author_id INTO v_author_id
      FROM billing_plan bp
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE bp.id = NEW.billing_plan_id;

    PERFORM 1 FROM "user" WHERE id = NEW.user_id FOR UPDATE;

    SELECT count(*) INTO v_conflict_count
      FROM subscription s
      JOIN billing_plan bp ON bp.id = s.billing_plan_id
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE s.user_id = NEW.user_id
       AND st.author_id = v_author_id
       AND s.status IN ('pending', 'active', 'paused')
       AND s.id <> NEW.id;

    IF v_conflict_count > 0 THEN
        RAISE EXCEPTION
            'user % already has a non-terminal subscription to author %',
            NEW.user_id, v_author_id
            USING ERRCODE = 'unique_violation',
                  CONSTRAINT = 'subscription_unique_non_terminal_user_author';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_unique_non_terminal
    BEFORE INSERT OR UPDATE OF billing_plan_id, status ON subscription
    FOR EACH ROW
    EXECUTE FUNCTION enforce_unique_non_terminal_subscription();