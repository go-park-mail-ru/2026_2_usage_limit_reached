DROP VIEW public."user";
DROP VIEW public.author;

ALTER TABLE identity."user" SET SCHEMA public;
ALTER TABLE identity.author SET SCHEMA public;

DROP SCHEMA identity;
