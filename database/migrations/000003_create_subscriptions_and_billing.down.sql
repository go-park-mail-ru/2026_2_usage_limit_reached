DROP TABLE IF EXISTS subscription CASCADE;
DROP TABLE IF EXISTS billing_plan CASCADE;
DROP TABLE IF EXISTS subscription_type CASCADE;

DROP FUNCTION IF EXISTS enforce_unique_non_terminal_subscription() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_plan_change_same_author() CASCADE;
DROP FUNCTION IF EXISTS check_subscription_not_self() CASCADE;