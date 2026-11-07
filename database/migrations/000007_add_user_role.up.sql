ALTER TABLE "user"
    ADD COLUMN role TEXT NOT NULL DEFAULT 'user',
    ADD CONSTRAINT chk_user_role
        CHECK (role IN ('user', 'admin', 'support'));