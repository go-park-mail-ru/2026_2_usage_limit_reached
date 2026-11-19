CREATE SCHEMA IF NOT EXISTS identity;

ALTER TABLE "user" SET SCHEMA identity;
ALTER TABLE author SET SCHEMA identity;

CREATE VIEW public."user" AS SELECT * FROM identity."user";
CREATE VIEW public.author AS SELECT * FROM identity.author;
