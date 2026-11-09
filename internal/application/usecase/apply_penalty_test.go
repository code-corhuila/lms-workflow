package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

// fakeRuns is an in-memory saga.RunRepository. It stores copies so a test
// observes only what was actually saved, not later in-memory mutations.
type fakeRuns struct {
	runs    map[string]saga.Run
	saves   int
	findErr error
	saveErr error
}

func newFakeRuns() *fakeRuns { return &fakeRuns{runs: map[string]saga.Run{}} }

func (f *fakeRuns) FindByLoanID(_ context.Context, loanID string) (*saga.Run, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	run, ok := f.runs[loanID]
	if !ok {
		return nil, saga.ErrRunNotFound
	}
	return &run, nil
}

func (f *fakeRuns) Save(_ context.Context, run *saga.Run) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saves++
	f.runs[run.LoanID] = *run
	return nil
}

type fakeCirculation struct {
	markLateCalls []string
	markLateErr   error
}

func (f *fakeCirculation) FindOverdueLoans(context.Context) ([]saga.OverdueLoan, error) {
	return nil, nil
}

func (f *fakeCirculation) MarkLate(_ context.Context, loanID string) error {
	f.markLateCalls = append(f.markLateCalls, loanID)
	return f.markLateErr
}

type fakeMembership struct {
	suspendCalls []string
	suspendErr   error
}

func (f *fakeMembership) Suspend(_ context.Context, studentID string) error {
	f.suspendCalls = append(f.suspendCalls, studentID)
	return f.suspendErr
}

var overdueLoan = saga.OverdueLoan{LoanID: "loan-1", StudentID: "student-1"}

func TestApplyPenaltyHappyPathCompletesRun(t *testing.T) {
	runs, circ, memb := newFakeRuns(), &fakeCirculation{}, &fakeMembership{}

	if err := NewApplyPenalty(runs, circ, memb).Execute(context.Background(), overdueLoan); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := runs.runs["loan-1"].Status; got != saga.StatusCompleted {
		t.Errorf("status = %q, want %q", got, saga.StatusCompleted)
	}
	if len(circ.markLateCalls) != 1 || circ.markLateCalls[0] != "loan-1" {
		t.Errorf("MarkLate calls = %v, want [loan-1]", circ.markLateCalls)
	}
	if len(memb.suspendCalls) != 1 || memb.suspendCalls[0] != "student-1" {
		t.Errorf("Suspend calls = %v, want [student-1]", memb.suspendCalls)
	}
}

func TestApplyPenaltySkipsCompletedRun(t *testing.T) {
	runs, circ, memb := newFakeRuns(), &fakeCirculation{}, &fakeMembership{}
	done := saga.NewRun("loan-1")
	done.Completed()
	runs.runs["loan-1"] = *done

	if err := NewApplyPenalty(runs, circ, memb).Execute(context.Background(), overdueLoan); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(circ.markLateCalls) != 0 || len(memb.suspendCalls) != 0 {
		t.Errorf("completed run re-applied steps: markLate=%v suspend=%v", circ.markLateCalls, memb.suspendCalls)
	}
	if runs.saves != 0 {
		t.Errorf("saves = %d, want 0", runs.saves)
	}
}

func TestApplyPenaltyResumesMarkedLateRunAtSuspend(t *testing.T) {
	runs, circ, memb := newFakeRuns(), &fakeCirculation{}, &fakeMembership{}
	stuck := saga.NewRun("loan-1")
	stuck.MarkedLate()
	runs.runs["loan-1"] = *stuck

	if err := NewApplyPenalty(runs, circ, memb).Execute(context.Background(), overdueLoan); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(circ.markLateCalls) != 0 {
		t.Errorf("MarkLate re-run on a MARKED_LATE run: %v", circ.markLateCalls)
	}
	if got := runs.runs["loan-1"].Status; got != saga.StatusCompleted {
		t.Errorf("status = %q, want %q", got, saga.StatusCompleted)
	}
}

func TestApplyPenaltyMarkLateFailureRecordsFailedRun(t *testing.T) {
	runs, memb := newFakeRuns(), &fakeMembership{}
	circ := &fakeCirculation{markLateErr: errors.New("circulation down")}

	err := NewApplyPenalty(runs, circ, memb).Execute(context.Background(), overdueLoan)
	if err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}

	saved := runs.runs["loan-1"]
	if saved.Status != saga.StatusFailed || saved.AttemptCount != 1 {
		t.Errorf("saved run = %+v, want FAILED with 1 attempt", saved)
	}
	if len(memb.suspendCalls) != 0 {
		t.Errorf("Suspend called after MarkLate failed: %v", memb.suspendCalls)
	}
}

// A suspend failure must never roll back, nor repeat, the already-applied
// mark-late step (ADR-008, forward-recovery only). The retry on the next
// tick must resume straight at the suspend step.
func TestApplyPenaltyRetryAfterSuspendFailureDoesNotRepeatMarkLate(t *testing.T) {
	runs, circ := newFakeRuns(), &fakeCirculation{}
	memb := &fakeMembership{suspendErr: errors.New("membership down")}
	uc := NewApplyPenalty(runs, circ, memb)

	if err := uc.Execute(context.Background(), overdueLoan); err == nil {
		t.Fatal("first Execute() error = nil, want the suspend error")
	}
	if saved := runs.runs["loan-1"]; saved.AttemptCount != 1 || saved.LastError == "" {
		t.Errorf("after suspend failure, saved run = %+v, want 1 attempt and a LastError", saved)
	}

	memb.suspendErr = nil
	if err := uc.Execute(context.Background(), overdueLoan); err != nil {
		t.Fatalf("retry Execute() error = %v", err)
	}

	if len(circ.markLateCalls) != 1 {
		t.Errorf("MarkLate called %d times across the retry, want exactly 1", len(circ.markLateCalls))
	}
	if got := runs.runs["loan-1"].Status; got != saga.StatusCompleted {
		t.Errorf("status after retry = %q, want %q", got, saga.StatusCompleted)
	}
}

func TestApplyPenaltyRetryAfterMarkLateFailureRunsBothSteps(t *testing.T) {
	runs, memb := newFakeRuns(), &fakeMembership{}
	circ := &fakeCirculation{markLateErr: errors.New("circulation down")}
	uc := NewApplyPenalty(runs, circ, memb)

	_ = uc.Execute(context.Background(), overdueLoan)
	circ.markLateErr = nil
	if err := uc.Execute(context.Background(), overdueLoan); err != nil {
		t.Fatalf("retry Execute() error = %v", err)
	}

	if len(circ.markLateCalls) != 2 || len(memb.suspendCalls) != 1 {
		t.Errorf("markLate=%d suspend=%d, want 2 and 1", len(circ.markLateCalls), len(memb.suspendCalls))
	}
	if got := runs.runs["loan-1"].Status; got != saga.StatusCompleted {
		t.Errorf("status after retry = %q, want %q", got, saga.StatusCompleted)
	}
}

func TestApplyPenaltyPropagatesRepositoryErrors(t *testing.T) {
	t.Run("find", func(t *testing.T) {
		runs := newFakeRuns()
		runs.findErr = errors.New("db down")
		circ := &fakeCirculation{}

		if err := NewApplyPenalty(runs, circ, &fakeMembership{}).Execute(context.Background(), overdueLoan); err == nil {
			t.Fatal("Execute() error = nil, want an error")
		}
		if len(circ.markLateCalls) != 0 {
			t.Errorf("MarkLate called despite load failure: %v", circ.markLateCalls)
		}
	})

	t.Run("save new run", func(t *testing.T) {
		runs := newFakeRuns()
		runs.saveErr = errors.New("db down")
		circ := &fakeCirculation{}

		if err := NewApplyPenalty(runs, circ, &fakeMembership{}).Execute(context.Background(), overdueLoan); err == nil {
			t.Fatal("Execute() error = nil, want an error")
		}
		if len(circ.markLateCalls) != 0 {
			t.Errorf("MarkLate called although the run was never persisted: %v", circ.markLateCalls)
		}
	})
}
