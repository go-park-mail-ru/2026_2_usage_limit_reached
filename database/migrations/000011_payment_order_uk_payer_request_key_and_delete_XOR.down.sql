ALTER TABLE payment_order
    DROP CONSTRAINT uk_payment_order_payer_request_key;

ALTER TABLE payment_order
    ADD CONSTRAINT uk_payment_order_request_key
    UNIQUE (request_key);