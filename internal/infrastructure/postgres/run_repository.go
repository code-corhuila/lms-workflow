package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

// RunRepository implements saga.RunRepository against PostgreSQL — the only
// code allowed to touch the `workflow.saga_run` table
// (library-docs/09-microservices/service-boundary-rules.md).
type RunRepository struct {
	db *pgxpool.Pool
}

func NewRunRepository(db *pgxpool.Pool) *RunRepository {
	return &RunRepository{db: db}
}

func (r *RunRepository) FindByLoanID(ctx context.Context, loanID string) (*saga.Run, error) {
	const query = `
		SELECT loan_id, status, attempt_count, last_error, created_at, updated_at
		FROM workflow.saga_run
		WHERE loan_id = $1`

	var run saga.Run
	var lastError *string
	err := r.db.QueryRow(ctx, query, loanID).Scan(
		&run.LoanID, &run.Status, &run.AttemptCount, &lastError, &run.CreatedAt, &run.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, saga.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastError != nil {
		run.LastError = *lastError
	}
	return &run, nil
}

func (r *RunRepository) Save(ctx context.Context, run *saga.Run) error {
	const query = `
		INSERT INTO workflow.saga_run (loan_id, status, attempt_count, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		ON CONFLICT (loan_id) DO UPDATE SET
			status = EXCLUDED.status,
			attempt_count = EXCLUDED.attempt_count,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at`

	_, err := r.db.Exec(ctx, query,
		run.LoanID, run.Status, run.AttemptCount, run.LastError, run.CreatedAt, run.UpdatedAt,
	)
	return err
}
