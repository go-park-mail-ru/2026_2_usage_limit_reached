-- Откат дефолтных прав для роли приложения (если она существует)
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE ON TABLES FROM app_user;
        REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM app_user;
        REVOKE USAGE ON SCHEMA public FROM app_user;
    END IF;
END
$$;

-- Удаление представлений
DROP VIEW IF EXISTS subscription_period_with_end CASCADE;

-- Удаление таблиц
DROP TABLE IF EXISTS notification CASCADE;
DROP TABLE IF EXISTS comment_like CASCADE;
DROP TABLE IF EXISTS "comment" CASCADE;
DROP TABLE IF EXISTS post_like CASCADE;
DROP TABLE IF EXISTS post_block CASCADE;
DROP TABLE IF EXISTS post_access CASCADE;
DROP TABLE IF EXISTS post CASCADE;
DROP TABLE IF EXISTS subscription_period CASCADE;
DROP TABLE IF EXISTS donation CASCADE;
DROP TABLE IF EXISTS chargeback CASCADE;
DROP TABLE IF EXISTS refund CASCADE;
DROP TABLE IF EXISTS provider_webhook_event CASCADE;
DROP TABLE IF EXISTS payment_attempt CASCADE;
DROP TABLE IF EXISTS payment_order CASCADE;
DROP TABLE IF EXISTS subscription CASCADE;
DROP TABLE IF EXISTS billing_plan CASCADE;
DROP TABLE IF EXISTS subscription_type CASCADE;
DROP TABLE IF EXISTS author CASCADE;
DROP TABLE IF EXISTS "user" CASCADE;

-- Удаление функций
DROP FUNCTION IF EXISTS check_payment_order_subject() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_period_author_matches() CASCADE;
DROP FUNCTION IF EXISTS enforce_unique_external_dispute_id() CASCADE;
DROP FUNCTION IF EXISTS enforce_unique_external_refund_id() CASCADE;
DROP FUNCTION IF EXISTS enforce_unique_non_terminal_subscription() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_plan_change_same_author() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_not_self() CASCADE;
DROP FUNCTION IF EXISTS compute_subscription_period_end(TIMESTAMPTZ, TEXT, SMALLINT) CASCADE;
DROP FUNCTION IF EXISTS reject_delete() CASCADE;
DROP FUNCTION IF EXISTS reject_update() CASCADE;
DROP FUNCTION IF EXISTS set_updated_at() CASCADE;