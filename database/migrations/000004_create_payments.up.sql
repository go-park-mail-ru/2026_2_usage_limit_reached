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