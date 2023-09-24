-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "description" character varying NULL, ADD COLUMN "embedding" vector(1536) NULL;
