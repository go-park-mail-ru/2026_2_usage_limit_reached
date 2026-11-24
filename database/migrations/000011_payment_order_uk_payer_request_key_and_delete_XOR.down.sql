ALTER TABLE payment_order
    DROP CONSTRAINT uk_payment_order_payer_request_key;

ALTER TABLE payment_order
    ADD CONSTRAINT uk_payment_order_request_key
    UNIQUE (request_key);

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