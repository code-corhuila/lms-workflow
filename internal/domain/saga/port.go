package saga

import (
	"context"
	"errors"
)

// ErrRunNotFound is the port-level "not found" error — use cases depend on
// this, never on an infrastructure package's own error type
// (library-docs/05-architecture/hexagonal-architecture.md, Dependency Rule).
var ErrRunNotFound = errors.New("saga run not found")

// RunRepository is the driven (secondary) port — implemented by
// internal/infrastructure/postgres.
type RunRepository interface {
	FindByLoanID(ctx context.Context, loanID string) (*Run, error)
	Save(ctx context.Context, run *Run) error
}

// OverdueLoan is the shape ApplyPenalty needs from a loan found by
// lms-circulation-api's overdue query — this saga never reads
// lms-circulation-api's database directly
// (library-docs/09-microservices/service-boundary-rules.md).
type OverdueLoan struct {
	LoanID    string
	StudentID string
}

// CirculationClient marks a loan late — step one.
type CirculationClient interface {
	FindOverdueLoans(ctx context.Context) ([]OverdueLoan, error)
	MarkLate(ctx context.Context, loanID string) error
}

// MembershipClient suspends the student tied to a late loan — step two.
type MembershipClient interface {
	Suspend(ctx context.Context, studentID string) error
}
