package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

// Integration tests: they run only when WORKFLOW_TEST_DATABASE_URL points at
// a PostgreSQL database where migrations/ has already been applied, e.g.
//
//	WORKFLOW_TEST_DATABASE_URL=postgres://workflow_app:pass@localhost:5432/lms_db?sslmode=disable go test ./...
//
// They need only the workflow_writer grants (SELECT, INSERT, UPDATE), so
// they can run as the real application user. Each test uses its own unique
// loan_id instead of truncating the table, which that user cannot do.
func newTestRepository(t *testing.T) (*RunRepository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("WORKFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("WORKFLOW_TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRunRepository(pool), pool
}

// uniqueLoanID keeps tests independent across runs against the same database.
func uniqueLoanID(t *testing.T) string {
	return fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
}

func TestRunRepositoryFindMissingReturnsErrRunNotFound(t *testing.T) {
	repo, _ := newTestRepository(t)

	_, err := repo.FindByLoanID(context.Background(), uniqueLoanID(t))
	if !errors.Is(err, saga.ErrRunNotFound) {
		t.Errorf("FindByLoanID() error = %v, want saga.ErrRunNotFound", err)
	}
}

func TestRunRepositorySaveAndFindRoundTrip(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	run := saga.NewRun(uniqueLoanID(t))

	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByLoanID(ctx, run.LoanID)
	if err != nil {
		t.Fatalf("FindByLoanID() error = %v", err)
	}
	if got.Status != saga.StatusPending || got.AttemptCount != 0 || got.LastError != "" {
		t.Errorf("got %+v, want PENDING, 0 attempts, no error", got)
	}
	if !got.CreatedAt.Equal(run.CreatedAt.Truncate(time.Microsecond)) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, run.CreatedAt)
	}
}

func TestRunRepositorySaveUpsertsExistingRun(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	run := saga.NewRun(uniqueLoanID(t))
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}

	run.MarkedLate()
	run.RecordFailure(errors.New("membership down"))
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	got, err := repo.FindByLoanID(ctx, run.LoanID)
	if err != nil {
		t.Fatalf("FindByLoanID() error = %v", err)
	}
	if got.Status != saga.StatusMarkedLate || got.AttemptCount != 1 || got.LastError != "membership down" {
		t.Errorf("got %+v, want MARKED_LATE, 1 attempt, LastError %q", got, "membership down")
	}
	if !got.CreatedAt.Equal(run.CreatedAt.Truncate(time.Microsecond)) {
		t.Errorf("upsert changed CreatedAt: %v, want %v", got.CreatedAt, run.CreatedAt)
	}
}

func TestRunRepositoryEmptyLastErrorStoredAsNull(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	run := saga.NewRun(uniqueLoanID(t))
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var isNull bool
	if err := pool.QueryRow(ctx, "SELECT last_error IS NULL FROM workflow.saga_run WHERE loan_id = $1", run.LoanID).Scan(&isNull); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !isNull {
		t.Error("last_error stored as empty string, want NULL")
	}
}

func TestRunRepositoryRejectsUnknownStatus(t *testing.T) {
	repo, _ := newTestRepository(t)
	run := saga.NewRun(uniqueLoanID(t))
	run.Status = "BOGUS"

	if err := repo.Save(context.Background(), run); err == nil {
		t.Error("Save() error = nil, want the status CHECK constraint to reject BOGUS")
	}
}
