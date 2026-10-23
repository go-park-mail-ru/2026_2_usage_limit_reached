DROP VIEW IF EXISTS subscription_period_with_end CASCADE;

DROP TABLE IF EXISTS subscription_period CASCADE;
DROP TABLE IF EXISTS donation CASCADE;

DROP FUNCTION IF EXISTS check_payment_order_subject() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_period_dates_immutable() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_period_author_matches() CASCADE;