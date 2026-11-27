ALTER TABLE subscription_period
    ADD COLUMN request_action TEXT,
    ADD COLUMN expected_current_tier_id UUID,
    ADD COLUMN post_id UUID,
    ADD CONSTRAINT fk_subscription_period_expected_current_tier
        FOREIGN KEY (expected_current_tier_id)
        REFERENCES subscription_type (id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT chk_subscription_period_request_metadata
        CHECK (
            (request_action IS NULL AND expected_current_tier_id IS NULL AND post_id IS NULL)
            OR (request_action IS NOT NULL AND request_action = 'subscribe' AND expected_current_tier_id IS NULL)
            OR (request_action IS NOT NULL AND request_action = 'upgrade' AND expected_current_tier_id IS NOT NULL)
        );

DROP TRIGGER trg_subscription_period_immutable ON subscription_period;

CREATE TRIGGER trg_subscription_period_immutable
    BEFORE UPDATE ON subscription_period
    FOR EACH ROW
    WHEN (
        OLD.payment_order_id IS DISTINCT FROM NEW.payment_order_id
        OR OLD.subscription_id IS DISTINCT FROM NEW.subscription_id
        OR OLD.billing_plan_id IS DISTINCT FROM NEW.billing_plan_id
        OR OLD.request_action IS DISTINCT FROM NEW.request_action
        OR OLD.expected_current_tier_id IS DISTINCT FROM NEW.expected_current_tier_id
        OR OLD.post_id IS DISTINCT FROM NEW.post_id
    )
    EXECUTE FUNCTION reject_update();
