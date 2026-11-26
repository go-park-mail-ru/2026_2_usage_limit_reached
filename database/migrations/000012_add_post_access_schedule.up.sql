ALTER TABLE post
    ADD COLUMN publication_ends_at TIMESTAMPTZ;

CREATE TABLE post_access_stage (
    post_id                UUID NOT NULL,
    starts_at              TIMESTAMPTZ NOT NULL,
    subscription_type_id   UUID,
    CONSTRAINT pk_post_access_stage
        PRIMARY KEY (post_id, starts_at),
    CONSTRAINT fk_post_access_stage_post
        FOREIGN KEY (post_id)
        REFERENCES post (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_post_access_stage_type
        FOREIGN KEY (subscription_type_id)
        REFERENCES subscription_type (id)
        ON DELETE RESTRICT
);

CREATE INDEX post_access_stage_subscription_type_id_idx
    ON post_access_stage (subscription_type_id);

CREATE OR REPLACE FUNCTION validate_post_access_stage_before_end()
RETURNS TRIGGER AS $$
DECLARE
    v_post_id UUID;
    v_publication_ends_at TIMESTAMPTZ;
BEGIN
    IF TG_TABLE_NAME = 'post' THEN
        v_post_id := NEW.id;
    ELSIF TG_OP = 'DELETE' THEN
        v_post_id := OLD.post_id;
    ELSE
        v_post_id := NEW.post_id;
    END IF;

    SELECT publication_ends_at
      INTO v_publication_ends_at
      FROM post
     WHERE id = v_post_id
     FOR UPDATE;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    IF v_publication_ends_at IS NOT NULL
       AND EXISTS (
            SELECT 1
              FROM post_access_stage pas
             WHERE pas.post_id = v_post_id
               AND pas.starts_at >= v_publication_ends_at
       )
    THEN
        RAISE EXCEPTION
            'post % has an access stage at or after publication_ends_at',
            v_post_id
            USING ERRCODE = 'check_violation',
                  CONSTRAINT = 'post_access_stage_before_publication_end';
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_post_access_stage_validate
    AFTER INSERT OR UPDATE OR DELETE ON post_access_stage
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION validate_post_access_stage_before_end();

CREATE CONSTRAINT TRIGGER trg_post_access_post_validate
    AFTER UPDATE OF publication_ends_at ON post
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION validate_post_access_stage_before_end();
