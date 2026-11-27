DROP TRIGGER trg_subscription_period_immutable ON subscription_period;

ALTER TABLE subscription_period
    DROP CONSTRAINT chk_subscription_period_request_metadata,
    DROP CONSTRAINT fk_subscription_period_expected_current_tier,
    DROP COLUMN request_action,
    DROP COLUMN expected_current_tier_id,
    DROP COLUMN post_id;

CREATE TRIGGER trg_subscription_period_immutable
    BEFORE UPDATE ON subscription_period
    FOR EACH ROW
    WHEN (
        OLD.payment_order_id IS DISTINCT FROM NEW.payment_order_id
        OR OLD.subscription_id IS DISTINCT FROM NEW.subscription_id
        OR OLD.billing_plan_id IS DISTINCT FROM NEW.billing_plan_id
    )
    EXECUTE FUNCTION reject_update();
