DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
        GRANT DELETE ON
            subscription,
            payment_order,
            payment_attempt,
            provider_webhook_event,
            refund,
            chargeback,
            subscription_period,
            donation
        TO app_user;
    END IF;
END
$$;