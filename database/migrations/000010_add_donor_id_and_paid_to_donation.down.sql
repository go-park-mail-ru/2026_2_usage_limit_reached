DROP INDEX IF EXISTS idx_donation_donor_feed;
DROP INDEX IF EXISTS idx_donation_author_feed;

ALTER TABLE donation
    DROP COLUMN IF EXISTS donor_id;