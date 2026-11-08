-- This domain's schema, on the same shared PostgreSQL instance every other
-- domain's schema lives on (Anexo J) — not a new database server, just a
-- new schema. Nothing lives in public — rules/2-anexos/A-db-postgres.md, rule 9.
CREATE SCHEMA IF NOT EXISTS workflow;
