CREATE INDEX idx_payment_order_expires_at_active
    ON payment_order (expires_at, id)
    WHERE status IN ('pending', 'processing')
      AND expires_at IS NOT NULL;