DROP INDEX uk_billing_plan_active_terms;
CREATE UNIQUE INDEX uk_billing_plan_active_terms
    ON billing_plan (subscription_type_id, interval_unit, interval_count, currency)
    WHERE retired_at IS NULL;

DROP TRIGGER trg_billing_plan_immutable ON billing_plan;
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

ALTER TABLE subscription_type
    ADD COLUMN level SMALLINT;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM subscription_type
        WHERE retired_at IS NULL
        GROUP BY author_id
        HAVING count(*) > 32767
    ) THEN
        RAISE EXCEPTION 'cannot restore SMALLINT subscription levels: an author has more than 32767 active tiers';
    END IF;
END;
$$;

WITH active_tier_prices AS (
    SELECT
        st.id,
        st.author_id,
        COALESCE(bp.amount_minor, 0) AS amount_minor
    FROM subscription_type st
    LEFT JOIN LATERAL (
        SELECT amount_minor
        FROM billing_plan
        WHERE subscription_type_id = st.id
          AND interval_unit = 'day'
          AND interval_count = 30
          AND currency = 'RUB'
        ORDER BY retired_at NULLS FIRST, created_at DESC, id DESC
        LIMIT 1
    ) bp ON TRUE
    WHERE st.retired_at IS NULL
), tier_levels AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY author_id
            ORDER BY amount_minor, id
        )::SMALLINT AS level
    FROM active_tier_prices
    UNION ALL
    SELECT id, 1::SMALLINT
    FROM subscription_type
    WHERE retired_at IS NOT NULL
)
UPDATE subscription_type st
   SET level = tier_levels.level
  FROM tier_levels
 WHERE st.id = tier_levels.id;

ALTER TABLE subscription_type
    ALTER COLUMN level SET NOT NULL,
    ADD CONSTRAINT chk_subscription_type_level CHECK (level >= 1);

CREATE UNIQUE INDEX uk_subscription_type_active_level
    ON subscription_type (author_id, level)
    WHERE retired_at IS NULL;

DROP TRIGGER trg_subscription_type_immutable ON subscription_type;
CREATE TRIGGER trg_subscription_type_immutable
    BEFORE UPDATE ON subscription_type
    FOR EACH ROW
    WHEN (
        OLD.author_id IS DISTINCT FROM NEW.author_id
        OR OLD.level IS DISTINCT FROM NEW.level
    )
    EXECUTE FUNCTION reject_update();
