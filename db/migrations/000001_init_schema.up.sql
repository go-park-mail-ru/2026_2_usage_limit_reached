-- Автообновление updated_at при изменении строки
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Отклоняет UPDATE защищённых значений
CREATE OR REPLACE FUNCTION reject_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'table "%": update of immutable data is not allowed',
        TG_TABLE_NAME
        USING ERRCODE = 'integrity_constraint_violation';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Запрещает физическое удаление финансовых записей
CREATE OR REPLACE FUNCTION reject_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'table "%": physical delete of financial records is not allowed',
        TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Конец расчётного периода по зафиксированному тарифу
CREATE OR REPLACE FUNCTION compute_subscription_period_end(
    p_period_start   TIMESTAMPTZ,
    p_interval_unit  TEXT,
    p_interval_count SMALLINT
) RETURNS TIMESTAMPTZ AS $$
BEGIN
    RETURN (
        (p_period_start AT TIME ZONE 'UTC')
        + CASE p_interval_unit
            WHEN 'day'   THEN make_interval(days  => p_interval_count)
            WHEN 'month' THEN make_interval(months => p_interval_count)
            WHEN 'year'  THEN make_interval(years  => p_interval_count)
          END
    ) AT TIME ZONE 'UTC';
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- =====================================================================
-- Пользователи и авторы
-- =====================================================================

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

-- =====================================================================
-- Уровни подписки, тарифы, договоры
-- =====================================================================

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

-- =====================================================================
-- Платежи
-- =====================================================================

-- Платёжное обязательство за период подписки или пожертвование
CREATE TABLE payment_order (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payer_id               UUID NOT NULL,
    request_key            TEXT NOT NULL,
    purpose                TEXT NOT NULL,
    origin                 TEXT NOT NULL,
    expected_amount_minor  BIGINT NOT NULL,
    currency               TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'pending',
    credited_attempt_id    UUID,
    expires_at             TIMESTAMPTZ,
    paid_at                TIMESTAMPTZ,
    canceled_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_payment_order_payer
        FOREIGN KEY (payer_id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT uk_payment_order_request_key
        UNIQUE (request_key),
    CONSTRAINT chk_payment_order_amount
        CHECK (expected_amount_minor > 0),
    CONSTRAINT chk_payment_order_currency
        CHECK (length(currency) = 3),
    CONSTRAINT chk_payment_order_purpose
        CHECK (purpose IN ('subscription_period', 'donation')),
    CONSTRAINT chk_payment_order_origin
        CHECK (origin IN ('manual', 'auto_renewal')),
    CONSTRAINT chk_payment_order_status
        CHECK (status IN ('pending', 'processing', 'paid', 'failed', 'canceled')),
    CONSTRAINT chk_payment_order_paid_at
        CHECK ((status = 'paid') = (paid_at IS NOT NULL)),
    CONSTRAINT chk_payment_order_credited_attempt
        CHECK ((status = 'paid') = (credited_attempt_id IS NOT NULL)),
    CONSTRAINT chk_payment_order_canceled_at
        CHECK ((status = 'canceled') = (canceled_at IS NOT NULL))
);

CREATE INDEX payment_order_payer_id_idx ON payment_order (payer_id);

CREATE TRIGGER trg_payment_order_immutable
    BEFORE UPDATE ON payment_order
    FOR EACH ROW
    WHEN (
        OLD.payer_id IS DISTINCT FROM NEW.payer_id
        OR OLD.request_key IS DISTINCT FROM NEW.request_key
        OR OLD.purpose IS DISTINCT FROM NEW.purpose
        OR OLD.origin IS DISTINCT FROM NEW.origin
        OR OLD.expected_amount_minor IS DISTINCT FROM NEW.expected_amount_minor
        OR OLD.currency IS DISTINCT FROM NEW.currency
        OR OLD.expires_at IS DISTINCT FROM NEW.expires_at
        OR (
            OLD.paid_at IS NOT NULL
            AND (
                OLD.paid_at IS DISTINCT FROM NEW.paid_at
                OR OLD.credited_attempt_id IS DISTINCT FROM NEW.credited_attempt_id
            )
        )
        OR (
            OLD.canceled_at IS NOT NULL
            AND OLD.canceled_at IS DISTINCT FROM NEW.canceled_at
        )
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_payment_order_no_delete
    BEFORE DELETE ON payment_order
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_payment_order_set_updated_at
    BEFORE UPDATE ON payment_order
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Отдельная попытка провести платёж через внешнего провайдера
CREATE TABLE payment_attempt (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_order_id         UUID NOT NULL,
    provider                 TEXT NOT NULL,
    external_transaction_id  TEXT,
    status                   TEXT NOT NULL DEFAULT 'created',
    charged_amount_minor     BIGINT,
    charged_currency         TEXT,
    failure_reason           TEXT,
    completed_at             TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_payment_attempt_order
        FOREIGN KEY (payment_order_id)
        REFERENCES payment_order (id)
        ON DELETE RESTRICT,
    CONSTRAINT uk_payment_attempt_provider_tx
        UNIQUE (provider, external_transaction_id),
    CONSTRAINT uk_payment_attempt_credit_target
        UNIQUE (payment_order_id, id, charged_amount_minor, charged_currency),
    CONSTRAINT uk_payment_attempt_refund_target
        UNIQUE (id, charged_amount_minor),
    CONSTRAINT chk_payment_attempt_status
        CHECK (status IN ('created', 'processing', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT chk_payment_attempt_succeeded_fields
        CHECK (
            status <> 'succeeded'
            OR (external_transaction_id IS NOT NULL AND completed_at IS NOT NULL)
        ),
    CONSTRAINT chk_payment_attempt_charged_amount_status
        CHECK ((status = 'succeeded') = (charged_amount_minor IS NOT NULL)),
    CONSTRAINT chk_payment_attempt_charged_currency_status
        CHECK ((status = 'succeeded') = (charged_currency IS NOT NULL)),
    CONSTRAINT chk_payment_attempt_charged_amount_positive
        CHECK (charged_amount_minor IS NULL OR charged_amount_minor > 0),
    CONSTRAINT chk_payment_attempt_charged_currency_len
        CHECK (charged_currency IS NULL OR length(charged_currency) = 3),
    CONSTRAINT chk_payment_attempt_failure_reason
        CHECK ((status IN ('failed', 'canceled')) = (failure_reason IS NOT NULL))
);

CREATE TRIGGER trg_payment_attempt_immutable
    BEFORE UPDATE ON payment_attempt
    FOR EACH ROW
    WHEN (
        OLD.payment_order_id IS DISTINCT FROM NEW.payment_order_id
        OR OLD.provider IS DISTINCT FROM NEW.provider
        OR (
            OLD.status = 'succeeded'
            AND (
                NEW.status IS DISTINCT FROM OLD.status
                OR OLD.external_transaction_id IS DISTINCT FROM NEW.external_transaction_id
                OR OLD.charged_amount_minor IS DISTINCT FROM NEW.charged_amount_minor
                OR OLD.charged_currency IS DISTINCT FROM NEW.charged_currency
                OR OLD.completed_at IS DISTINCT FROM NEW.completed_at
            )
        )
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_payment_attempt_no_delete
    BEFORE DELETE ON payment_attempt
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_payment_attempt_set_updated_at
    BEFORE UPDATE ON payment_attempt
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Зачтённая попытка: составной FK гарантирует, что попытка
--   1) принадлежит именно этому заказу (payment_order_id = id заказа),
--   2) успешна (charged_* заполнены только у succeeded),
--   3) списала ровно ожидаемые сумму и валюту.
ALTER TABLE payment_order
    ADD CONSTRAINT fk_payment_order_credited_attempt
        FOREIGN KEY (id, credited_attempt_id, expected_amount_minor, currency)
        REFERENCES payment_attempt (payment_order_id, id, charged_amount_minor, charged_currency)
        ON DELETE RESTRICT;

CREATE INDEX payment_order_credited_attempt_id_idx ON payment_order (credited_attempt_id);

-- Учёт полученных событий провайдера для идемпотентной обработки вебхуков
CREATE TABLE provider_webhook_event (
    provider            TEXT NOT NULL,
    external_event_id   TEXT NOT NULL,
    event_kind          TEXT NOT NULL,
    processed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pk_provider_webhook_event
        PRIMARY KEY (provider, external_event_id),
    CONSTRAINT chk_provider_webhook_event_kind_not_blank
        CHECK (length(trim(event_kind)) > 0)
);

CREATE TRIGGER trg_provider_webhook_event_immutable
    BEFORE UPDATE ON provider_webhook_event
    FOR EACH ROW
    WHEN (
        OLD.provider IS DISTINCT FROM NEW.provider
        OR OLD.external_event_id IS DISTINCT FROM NEW.external_event_id
        OR OLD.event_kind IS DISTINCT FROM NEW.event_kind
        OR (
            OLD.processed_at IS NOT NULL
            AND OLD.processed_at IS DISTINCT FROM NEW.processed_at
        )
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_provider_webhook_event_no_delete
    BEFORE DELETE ON provider_webhook_event
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_provider_webhook_event_set_updated_at
    BEFORE UPDATE ON provider_webhook_event
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Запрос и результат полного возврата одного подтверждённого списания
CREATE TABLE refund (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_attempt_id   UUID NOT NULL,
    external_refund_id   TEXT,
    amount_minor         BIGINT NOT NULL,
    kind                 TEXT NOT NULL,
    reason               TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'pending',
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_refund_payment_attempt
        UNIQUE (payment_attempt_id),
    CONSTRAINT fk_refund_attempt_amount
        FOREIGN KEY (payment_attempt_id, amount_minor)
        REFERENCES payment_attempt (id, charged_amount_minor)
        ON DELETE RESTRICT,
    CONSTRAINT chk_refund_amount_positive
        CHECK (amount_minor > 0),
    CONSTRAINT chk_refund_kind
        CHECK (kind IN ('customer_request', 'excess_charge')),
    CONSTRAINT chk_refund_status
        CHECK (status IN ('pending', 'processing', 'succeeded', 'failed')),
    CONSTRAINT chk_refund_succeeded_completed_at
        CHECK ((status = 'succeeded') = (completed_at IS NOT NULL)),
    CONSTRAINT chk_refund_succeeded_external_id
        CHECK (status <> 'succeeded' OR external_refund_id IS NOT NULL)
);

-- Ускоряет проверку уникальности внешнего идентификатора возврата
CREATE INDEX refund_external_refund_id_idx
    ON refund (external_refund_id)
    WHERE external_refund_id IS NOT NULL;

CREATE TRIGGER trg_refund_immutable
    BEFORE UPDATE ON refund
    FOR EACH ROW
    WHEN (
        OLD.payment_attempt_id IS DISTINCT FROM NEW.payment_attempt_id
        OR OLD.amount_minor IS DISTINCT FROM NEW.amount_minor
        OR OLD.kind IS DISTINCT FROM NEW.kind
        OR OLD.reason IS DISTINCT FROM NEW.reason
        OR (
            OLD.status = 'succeeded'
            AND (
                NEW.status IS DISTINCT FROM OLD.status
                OR OLD.external_refund_id IS DISTINCT FROM NEW.external_refund_id
                OR OLD.completed_at IS DISTINCT FROM NEW.completed_at
            )
        )
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_refund_no_delete
    BEFORE DELETE ON refund
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_refund_set_updated_at
    BEFORE UPDATE ON refund
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- external_refund_id уникален в рамках провайдера попытки.
-- Проверка сериализуется advisory-блокировкой по паре {provider, id}:
-- второй писатель ждёт коммита первого и видит его строку
CREATE OR REPLACE FUNCTION enforce_unique_external_refund_id()
RETURNS TRIGGER AS $$
DECLARE
    v_provider TEXT;
BEGIN
    SELECT provider INTO v_provider
      FROM payment_attempt
     WHERE id = NEW.payment_attempt_id;

    IF v_provider IS NULL THEN
        RETURN NEW; -- несуществующую попытку отклонит FK
    END IF;

    PERFORM pg_advisory_xact_lock( -- advisory-блокировка по провайдеру и id спора
        hashtextextended('refund:' || v_provider || ':' || NEW.external_refund_id, 0)
    );

    IF EXISTS (
        SELECT 1
          FROM refund r
          JOIN payment_attempt a ON a.id = r.payment_attempt_id
         WHERE a.provider = v_provider
           AND r.external_refund_id = NEW.external_refund_id
           AND r.id <> NEW.id
    ) THEN
        RAISE EXCEPTION 'external_refund_id % is already registered for provider %',
            NEW.external_refund_id, v_provider
            USING ERRCODE = 'unique_violation',
                  CONSTRAINT = 'uk_refund_provider_external_refund_id';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_refund_unique_external_id
    BEFORE INSERT OR UPDATE OF external_refund_id, payment_attempt_id ON refund
    FOR EACH ROW
    WHEN (NEW.external_refund_id IS NOT NULL)
    EXECUTE FUNCTION enforce_unique_external_refund_id();

-- Спор о подтверждённом списании, открытый у провайдера
CREATE TABLE chargeback (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_attempt_id     UUID NOT NULL,
    external_dispute_id    TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'open',
    opened_at              TIMESTAMPTZ NOT NULL,
    resolved_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_chargeback_attempt
        FOREIGN KEY (payment_attempt_id)
        REFERENCES payment_attempt (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_chargeback_status
        CHECK (status IN ('open', 'won', 'lost')),
    CONSTRAINT chk_chargeback_resolved_matches_status
        CHECK ((status IN ('won', 'lost')) = (resolved_at IS NOT NULL)),
    CONSTRAINT chk_chargeback_resolved_after_opened
        CHECK (resolved_at IS NULL OR resolved_at >= opened_at)
);

CREATE INDEX chargeback_payment_attempt_id_idx ON chargeback (payment_attempt_id);

-- Ускоряет проверку уникальности внешнего идентификатора спора
CREATE INDEX chargeback_external_dispute_id_idx ON chargeback (external_dispute_id);

CREATE TRIGGER trg_chargeback_immutable
    BEFORE UPDATE ON chargeback
    FOR EACH ROW
    WHEN (
        OLD.payment_attempt_id IS DISTINCT FROM NEW.payment_attempt_id
        OR OLD.external_dispute_id IS DISTINCT FROM NEW.external_dispute_id
        OR OLD.opened_at IS DISTINCT FROM NEW.opened_at
        OR (
            OLD.status <> 'open'
            AND (
                NEW.status IS DISTINCT FROM OLD.status
                OR OLD.resolved_at IS DISTINCT FROM NEW.resolved_at
            )
        )
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_chargeback_no_delete
    BEFORE DELETE ON chargeback
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_chargeback_set_updated_at
    BEFORE UPDATE ON chargeback
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- external_dispute_id уникален в рамках провайдера попытки (аналогично с refund)
CREATE OR REPLACE FUNCTION enforce_unique_external_dispute_id()
RETURNS TRIGGER AS $$
DECLARE
    v_provider TEXT;
BEGIN
    SELECT provider INTO v_provider
      FROM payment_attempt
     WHERE id = NEW.payment_attempt_id;

    IF v_provider IS NULL THEN
        RETURN NEW; -- несуществующую попытку отклонит FK
    END IF;

    PERFORM pg_advisory_xact_lock(
        hashtextextended('chargeback:' || v_provider || ':' || NEW.external_dispute_id, 0)
    );

    IF EXISTS (
        SELECT 1
          FROM chargeback c
          JOIN payment_attempt a ON a.id = c.payment_attempt_id
         WHERE a.provider = v_provider
           AND c.external_dispute_id = NEW.external_dispute_id
           AND c.id <> NEW.id
    ) THEN
        RAISE EXCEPTION 'external_dispute_id % is already registered for provider %',
            NEW.external_dispute_id, v_provider
            USING ERRCODE = 'unique_violation',
                  CONSTRAINT = 'uk_chargeback_provider_external_dispute_id';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_chargeback_unique_external_id
    BEFORE INSERT OR UPDATE OF external_dispute_id, payment_attempt_id ON chargeback
    FOR EACH ROW
    EXECUTE FUNCTION enforce_unique_external_dispute_id();

-- =====================================================================
-- Донаты и период подписки
-- =====================================================================

-- Разовые донаты
CREATE TABLE donation (
    payment_order_id  UUID PRIMARY KEY,
    author_id         UUID NOT NULL,
    message           TEXT,
    is_anonymous      BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_donation_order
        FOREIGN KEY (payment_order_id)
        REFERENCES payment_order (id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_donation_author
        FOREIGN KEY (author_id)
        REFERENCES author (id)
        ON DELETE RESTRICT
);

CREATE INDEX donation_author_id_idx ON donation (author_id);

CREATE TRIGGER trg_donation_immutable
    BEFORE UPDATE ON donation
    FOR EACH ROW
    WHEN (
        OLD.payment_order_id IS DISTINCT FROM NEW.payment_order_id
        OR OLD.author_id IS DISTINCT FROM NEW.author_id
        OR OLD.message IS DISTINCT FROM NEW.message
        OR OLD.is_anonymous IS DISTINCT FROM NEW.is_anonymous
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_donation_no_delete
    BEFORE DELETE ON donation
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_donation_set_updated_at
    BEFORE UPDATE ON donation
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Покупка расчётного периода подписки
CREATE TABLE subscription_period (
    payment_order_id  UUID PRIMARY KEY,
    subscription_id   UUID NOT NULL,
    billing_plan_id   UUID NOT NULL,
    period_start      TIMESTAMPTZ NOT NULL,
    revoked_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_subscription_period_order
        FOREIGN KEY (payment_order_id)
        REFERENCES payment_order (id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_subscription_period_sub
        FOREIGN KEY (subscription_id)
        REFERENCES subscription (id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_subscription_period_plan
        FOREIGN KEY (billing_plan_id)
        REFERENCES billing_plan (id)
        ON DELETE RESTRICT
);

CREATE INDEX subscription_period_subscription_id_idx ON subscription_period (subscription_id);
CREATE INDEX subscription_period_billing_plan_id_idx ON subscription_period (billing_plan_id);

CREATE TRIGGER trg_subscription_period_immutable
    BEFORE UPDATE ON subscription_period
    FOR EACH ROW
    WHEN (
        OLD.payment_order_id IS DISTINCT FROM NEW.payment_order_id
        OR OLD.subscription_id IS DISTINCT FROM NEW.subscription_id
        OR OLD.billing_plan_id IS DISTINCT FROM NEW.billing_plan_id
    )
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_subscription_period_no_delete
    BEFORE DELETE ON subscription_period
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

CREATE TRIGGER trg_subscription_period_set_updated_at
    BEFORE UPDATE ON subscription_period
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Тариф периода должен принадлежать тому же автору, что и текущий тариф связанной подписки
CREATE OR REPLACE FUNCTION check_subscription_period_author_matches()
RETURNS TRIGGER AS $$
DECLARE
    v_period_author_id   UUID;
    v_current_author_id  UUID;
BEGIN
    SELECT st.author_id INTO v_period_author_id
      FROM billing_plan bp
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE bp.id = NEW.billing_plan_id;

    SELECT st.author_id INTO v_current_author_id
      FROM subscription s
      JOIN billing_plan bp ON bp.id = s.billing_plan_id
      JOIN subscription_type st ON st.id = bp.subscription_type_id
     WHERE s.id = NEW.subscription_id;

    IF v_period_author_id IS DISTINCT FROM v_current_author_id THEN
        RAISE EXCEPTION
            'subscription_period billing_plan author (%) does not match subscription current author (%)',
            v_period_author_id, v_current_author_id
            USING ERRCODE = 'check_violation',
                  CONSTRAINT = 'subscription_period_author_matches';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_period_author_matches
    BEFORE INSERT ON subscription_period
    FOR EACH ROW
    EXECUTE FUNCTION check_subscription_period_author_matches();

-- нельзя менять статус заказа после payment_order.status = 'paid'
CREATE OR REPLACE FUNCTION check_subscription_period_dates_immutable()
RETURNS TRIGGER AS $$
DECLARE
    v_order_status TEXT;
BEGIN
    IF OLD.period_start IS NOT DISTINCT FROM NEW.period_start THEN
        RETURN NEW;
    END IF;

    SELECT status INTO v_order_status
      FROM payment_order
     WHERE id = NEW.payment_order_id;

    IF v_order_status = 'paid' THEN
        RAISE EXCEPTION 'cannot change period_start of an already paid period (order %)',
            NEW.payment_order_id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_period_protect_dates
    BEFORE UPDATE OF period_start ON subscription_period
    FOR EACH ROW
    EXECUTE FUNCTION check_subscription_period_dates_immutable();

-- Вью с вычисленным period_end
CREATE VIEW subscription_period_with_end AS
SELECT
    sp.payment_order_id,
    sp.subscription_id,
    sp.billing_plan_id,
    sp.period_start,
    compute_subscription_period_end(sp.period_start, bp.interval_unit, bp.interval_count) AS period_end,
    sp.revoked_at,
    sp.created_at,
    sp.updated_at
FROM subscription_period sp
JOIN billing_plan bp ON bp.id = sp.billing_plan_id;

-- XOR предмета заказа
CREATE OR REPLACE FUNCTION check_payment_order_subject()
RETURNS TRIGGER AS $$
DECLARE
    v_order_id        UUID;
    v_purpose         TEXT;
    v_donation_count  INTEGER;
    v_period_count    INTEGER;
BEGIN
    IF TG_TABLE_NAME = 'payment_order' THEN
        v_order_id := NEW.id;
    ELSE
        v_order_id := NEW.payment_order_id;
    END IF;

    SELECT purpose INTO v_purpose
      FROM payment_order
     WHERE id = v_order_id;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT count(*) INTO v_donation_count
      FROM donation
     WHERE payment_order_id = v_order_id;

    SELECT count(*) INTO v_period_count
      FROM subscription_period
     WHERE payment_order_id = v_order_id;

    IF (v_purpose = 'donation' AND v_donation_count = 1 AND v_period_count = 0)
       OR (v_purpose = 'subscription_period' AND v_period_count = 1 AND v_donation_count = 0)
    THEN
        RETURN NULL;
    END IF;

    RAISE EXCEPTION 'payment order % must have exactly one subject matching purpose %',
        v_order_id, v_purpose
        USING ERRCODE = 'check_violation',
              CONSTRAINT = 'payment_order_single_subject';
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_payment_order_subject_check
    AFTER INSERT OR UPDATE OF purpose ON payment_order
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_payment_order_subject();

CREATE CONSTRAINT TRIGGER trg_donation_subject_check
    AFTER INSERT OR UPDATE OF payment_order_id ON donation
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_payment_order_subject();

CREATE CONSTRAINT TRIGGER trg_subscription_period_subject_check
    AFTER INSERT OR UPDATE OF payment_order_id ON subscription_period
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_payment_order_subject();

-- =====================================================================
-- Контент
-- =====================================================================

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

CREATE INDEX post_author_id_idx ON post (author_id);

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

CREATE INDEX post_access_subscription_type_id_idx ON post_access (subscription_type_id);

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

CREATE INDEX post_like_user_id_idx ON post_like (user_id);

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

-- =====================================================================
-- Настройка роли приложения
-- =====================================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
        CREATE ROLE app_user; -- пароль устонавливаем в docker
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT DELETE ON ALL TABLES IN SCHEMA public TO app_user;

-- отключаем права на удаление финансовых записей
REVOKE DELETE ON 
    subscription, 
    payment_order, 
    payment_attempt,
    provider_webhook_event, 
    refund, 
    chargeback, 
    subscription_period, 
    donation 
FROM app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE ON TABLES TO app_user;