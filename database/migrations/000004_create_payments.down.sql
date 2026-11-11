DROP TABLE IF EXISTS chargeback CASCADE;
DROP TABLE IF EXISTS refund CASCADE;
DROP TABLE IF EXISTS provider_webhook_event CASCADE;
DROP TABLE IF EXISTS payment_attempt CASCADE;
DROP TABLE IF EXISTS payment_order CASCADE;

DROP FUNCTION IF EXISTS enforce_unique_external_dispute_id() CASCADE;
DROP FUNCTION IF EXISTS enforce_unique_external_refund_id() CASCADE;