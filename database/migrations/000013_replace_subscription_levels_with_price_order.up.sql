DROP TRIGGER trg_subscription_type_immutable ON subscription_type;
DROP INDEX uk_subscription_type_active_level;
ALTER TABLE subscription_type
    DROP CONSTRAINT chk_subscription_type_level,
    DROP COLUMN level;

CREATE TRIGGER trg_subscription_type_immutable
    BEFORE UPDATE ON subscription_type
    FOR EACH ROW
    WHEN (OLD.author_id IS DISTINCT FROM NEW.author_id)
    EXECUTE FUNCTION reject_update();

DROP INDEX uk_billing_plan_active_terms;
CREATE UNIQUE INDEX uk_billing_plan_active_terms
    ON billing_plan (subscription_type_id, interval_unit, interval_count)
    WHERE retired_at IS NULL;

DROP TRIGGER trg_billing_plan_immutable ON billing_plan;
CREATE TRIGGER trg_billing_plan_immutable
    BEFORE UPDATE ON billing_plan
    FOR EACH ROW
    WHEN (
        OLD.subscription_type_id IS DISTINCT FROM NEW.subscription_type_id
        OR OLD.interval_unit IS DISTINCT FROM NEW.interval_unit
        OR OLD.interval_count IS DISTINCT FROM NEW.interval_count
        OR OLD.currency IS DISTINCT FROM NEW.currency
    )
    EXECUTE FUNCTION reject_update();

-- Prices are mutable; price ordering and uniqueness are validated in repository transactions.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM billing_plan bp
        JOIN subscription_type st ON st.id = bp.subscription_type_id
        WHERE bp.retired_at IS NULL
          AND st.retired_at IS NULL
        GROUP BY st.author_id, bp.interval_unit, bp.interval_count, bp.currency, bp.amount_minor
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'active subscription tiers have duplicate prices for the same billing interval';
    END IF;
END;
$$;
