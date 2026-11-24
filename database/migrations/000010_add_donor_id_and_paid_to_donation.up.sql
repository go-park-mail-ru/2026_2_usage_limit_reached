ALTER TABLE donation
    ADD COLUMN donor_id UUID;
ALTER TABLE donation
    ADD COLUMN paid BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE donation d
SET donor_id = po.payer_id
FROM payment_order po
WHERE po.id = d.payment_order_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM donation
        WHERE donor_id IS NULL
    ) THEN
        RAISE EXCEPTION
            'Cannot set donation.donor_id NOT NULL: some donations have no matching payment order';
    END IF;
END
$$;

ALTER TABLE donation
    ALTER COLUMN donor_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_donation_donor_feed
    ON donation (donor_id, created_at DESC, payment_order_id DESC);

CREATE INDEX IF NOT EXISTS idx_donation_author_feed
    ON donation (author_id, created_at DESC, payment_order_id DESC)
    WHERE paid = TRUE;