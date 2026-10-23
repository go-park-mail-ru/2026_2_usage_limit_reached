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