package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/code-corhuila/lms-workflow/internal/application/usecase"
	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

type memRuns struct {
	mu   sync.Mutex
	runs map[string]saga.Run
}

func (m *memRuns) FindByLoanID(_ context.Context, id string) (*saga.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, saga.ErrRunNotFound
	}
	return &r, nil
}

func (m *memRuns) Save(_ context.Context, r *saga.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[r.LoanID] = *r
	return nil
}

type stubCirculation struct {
	mu         sync.Mutex
	loans      []saga.OverdueLoan
	findErr    error
	findCalls  int
	markLateOK map[string]bool
}

func (s *stubCirculation) FindOverdueLoans(context.Context) ([]saga.OverdueLoan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findCalls++
	return s.loans, s.findErr
}

func (s *stubCirculation) MarkLate(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.markLateOK[id] {
		return errors.New("mark-late failed")
	}
	return nil
}

func (s *stubCirculation) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findCalls
}

type okMembership struct{}

func (okMembership) Suspend(context.Context, string) error { return nil }

func newTicker(interval time.Duration, circ *stubCirculation, runs *memRuns) *Ticker {
	uc := usecase.NewApplyPenalty(runs, circ, okMembership{})
	return NewTicker(interval, circ, uc, zap.NewNop().Sugar())
}

// One failing loan must not stop the tick from processing the others.
func TestTickProcessesEveryLoanDespiteFailures(t *testing.T) {
	runs := &memRuns{runs: map[string]saga.Run{}}
	circ := &stubCirculation{
		loans:      []saga.OverdueLoan{{LoanID: "bad", StudentID: "s1"}, {LoanID: "good", StudentID: "s2"}},
		markLateOK: map[string]bool{"good": true},
	}

	newTicker(time.Hour, circ, runs).tick(context.Background())

	if got := runs.runs["bad"].Status; got != saga.StatusFailed {
		t.Errorf("bad loan status = %q, want %q", got, saga.StatusFailed)
	}
	if got := runs.runs["good"].Status; got != saga.StatusCompleted {
		t.Errorf("good loan status = %q, want %q", got, saga.StatusCompleted)
	}
}

func TestTickSurvivesFindOverdueError(t *testing.T) {
	runs := &memRuns{runs: map[string]saga.Run{}}
	circ := &stubCirculation{findErr: errors.New("circulation down")}

	newTicker(time.Hour, circ, runs).tick(context.Background())

	if len(runs.runs) != 0 {
		t.Errorf("runs = %v, want none", runs.runs)
	}
}

func TestRunTicksImmediatelyRepeatsAndStopsOnCancel(t *testing.T) {
	runs := &memRuns{runs: map[string]saga.Run{}}
	circ := &stubCirculation{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		newTicker(10*time.Millisecond, circ, runs).Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for circ.calls() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n := circ.calls(); n < 3 {
		t.Fatalf("FindOverdueLoans called %d times, want at least 3 (immediate tick + interval ticks)", n)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
