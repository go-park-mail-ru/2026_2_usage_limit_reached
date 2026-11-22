CREATE SCHEMA IF NOT EXISTS payment;

ALTER TABLE payment_order SET SCHEMA payment;
ALTER TABLE payment_attempt SET SCHEMA payment;
ALTER TABLE refund SET SCHEMA payment;

CREATE VIEW public.payment_order AS SELECT * FROM payment.payment_order;
CREATE VIEW public.payment_attempt AS SELECT * FROM payment.payment_attempt;
CREATE VIEW public.refund AS SELECT * FROM payment.refund;
