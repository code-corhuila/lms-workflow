// Package scheduler drives ApplyPenalty on a schedule. This is the saga's
// own trigger — distinct from lms-worker (ADR-009), which only sends
// due-date notices and never applies penalties itself.
package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/code-corhuila/lms-workflow/internal/application/usecase"
	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

type Ticker struct {
	interval     time.Duration
	circulation  saga.CirculationClient
	applyPenalty *usecase.ApplyPenalty
	log          *zap.SugaredLogger
}

func NewTicker(interval time.Duration, circulation saga.CirculationClient, applyPenalty *usecase.ApplyPenalty, log *zap.SugaredLogger) *Ticker {
	return &Ticker{interval: interval, circulation: circulation, applyPenalty: applyPenalty, log: log}
}

// Run blocks until ctx is cancelled, driving one tick immediately and then
// every t.interval.
func (t *Ticker) Run(ctx context.Context) {
	t.tick(ctx)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.tick(ctx)
		}
	}
}

func (t *Ticker) tick(ctx context.Context) {
	loans, err := t.circulation.FindOverdueLoans(ctx)
	if err != nil {
		t.log.Errorw("finding overdue loans", "error", err)
		return
	}

	for _, loan := range loans {
		if err := t.applyPenalty.Execute(ctx, loan); err != nil {
			t.log.Errorw("applying penalty", "loanId", loan.LoanID, "error", err)
		}
	}
}
