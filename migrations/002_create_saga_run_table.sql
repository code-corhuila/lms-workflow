-- The saga's own durable state (ADR-008-circulation-saga-scope.md) — one row
-- per loan_id per penalty flow. loan_id is `text`, not `uuid`: it originates
-- from lms-circulation-db's MongoDB ObjectId (ADR-005), not a Postgres UUID.
CREATE TABLE workflow.saga_run (
  loan_id         text        PRIMARY KEY,
  status          text        NOT NULL CHECK (status IN ('PENDING','MARKED_LATE','COMPLETED','FAILED')),
  attempt_count   integer     NOT NULL DEFAULT 0,
  last_error      text,

  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
