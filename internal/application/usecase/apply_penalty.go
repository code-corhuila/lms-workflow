// Package usecase orchestrates the domain for the saga — it contains no
// business logic itself (that lives in the Aggregate), only coordination
// (library-docs/05-architecture/hexagonal-architecture.md).
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

// ApplyPenalty implements ADR-008's two-step saga for one overdue loan:
// mark it late in lms-circulation-api, then suspend the student in
// lms-membership-api. loan_id is the idempotency key — a run already
// COMPLETED is skipped entirely; a run stuck at MARKED_LATE resumes
// straight at the suspend step, never re-running MarkLate.
type ApplyPenalty struct {
	Runs        saga.RunRepository
	Circulation saga.CirculationClient
	Membership  saga.MembershipClient
}

func NewApplyPenalty(runs saga.RunRepository, circulation saga.CirculationClient, membership saga.MembershipClient) *ApplyPenalty {
	return &ApplyPenalty{Runs: runs, Circulation: circulation, Membership: membership}
}

func (uc *ApplyPenalty) Execute(ctx context.Context, loan saga.OverdueLoan) error {
	run, err := uc.Runs.FindByLoanID(ctx, loan.LoanID)
	if errors.Is(err, saga.ErrRunNotFound) {
		run = saga.NewRun(loan.LoanID)
		if err := uc.Runs.Save(ctx, run); err != nil {
			return fmt.Errorf("saving new saga run: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("loading saga run: %w", err)
	}

	// Already fully applied — the idempotency check (ADR-008).
	if run.Status == saga.StatusCompleted {
		return nil
	}

	// Step one — skipped on resume if already MARKED_LATE.
	if run.Status != saga.StatusMarkedLate {
		if err := uc.Circulation.MarkLate(ctx, loan.LoanID); err != nil {
			run.Failed(fmt.Errorf("marking loan late: %w", err))
			_ = uc.Runs.Save(ctx, run)
			return err
		}
		run.MarkedLate()
		if err := uc.Runs.Save(ctx, run); err != nil {
			return fmt.Errorf("saving saga run after mark-late: %w", err)
		}
	}

	// Step two — never rolled back on failure (forward-recovery only); a
	// retry resumes here directly since run.Status is already MARKED_LATE.
	if err := uc.Membership.Suspend(ctx, loan.StudentID); err != nil {
		run.Failed(fmt.Errorf("suspending student: %w", err))
		_ = uc.Runs.Save(ctx, run)
		return err
	}

	run.Completed()
	return uc.Runs.Save(ctx, run)
}
