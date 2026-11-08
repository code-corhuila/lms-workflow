-- NOLOGIN: these carry permissions only. The infrastructure creates the
-- actual login user from secrets and grants it one of these roles
-- (rules/2-anexos/A-db-postgres.md, rule 10).
CREATE ROLE workflow_reader NOLOGIN;
CREATE ROLE workflow_writer NOLOGIN;

GRANT USAGE ON SCHEMA workflow TO workflow_reader, workflow_writer;

GRANT SELECT ON workflow.saga_run TO workflow_reader;
GRANT SELECT, INSERT, UPDATE ON workflow.saga_run TO workflow_writer;

-- Ties the permission-carrying role to the actual login user — the role was
-- NOLOGIN on purpose above; the login user itself is created by
-- lms-infra-postgres's init script, not by this migration
-- (rules/3-Anexo-J, J.7).
GRANT workflow_writer TO workflow_app;
