-- The ticker's tick() only ever looks up by loan_id (the primary key, already
-- indexed) — this index is for an operator querying "what's stuck in a
-- non-terminal state" without a full table scan.
CREATE INDEX idx_saga_run_status ON workflow.saga_run (status);
