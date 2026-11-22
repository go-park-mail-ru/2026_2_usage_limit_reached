DROP VIEW public.payment_order;
DROP VIEW public.payment_attempt;
DROP VIEW public.refund;

ALTER TABLE payment.payment_order SET SCHEMA public;
ALTER TABLE payment.payment_attempt SET SCHEMA public;
ALTER TABLE payment.refund SET SCHEMA public;

DROP SCHEMA payment;
