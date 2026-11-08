// Package saga implements the overdue-detection + penalty-application saga
// (ADR-008-circulation-saga-scope.md) — NOT the full loan lifecycle
// (registration/return stay direct, synchronous calls between
// lms-circulation-api, lms-membership-api and lms-catalog-api).
package saga

import "time"

// Status — PENDING is the implicit starting state (a run row is only ever
// inserted once step one has actually happened, so a run never observes
// PENDING itself; it exists as a named state for completeness and for any
// future step that precedes "marked late").
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusMarkedLate Status = "MARKED_LATE"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
)

// Run is one row per loan_id per penalty flow — the saga's own durable
// state, per ADR-008 ("workflow_db"/"saga_runs"), superseding ADR-006's
// blanket "workflow is stateless" line for this one subsystem specifically.
//
// LoanID is the saga's idempotency key: a run is looked up by LoanID before
// doing anything, so a retried trigger (lms-worker's ticker finding the same
// overdue loan twice) never re-applies a step that already succeeded.
type Run struct {
	LoanID       string
	Status       Status
	AttemptCount int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewRun starts a fresh saga run for a loan that was just found overdue.
func NewRun(loanID string) *Run {
	now := time.Now().UTC()
	return &Run{
		LoanID:       loanID,
		Status:       StatusPending,
		AttemptCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// MarkedLate records that step one (mark the loan late in
// lms-circulation-api) succeeded — forward-recovery only: this state is
// never rolled back if step two (suspend in lms-membership-api) later fails.
func (r *Run) MarkedLate() {
	r.Status = StatusMarkedLate
	r.UpdatedAt = time.Now().UTC()
}

// Completed records that both steps succeeded — a run in this state is
// skipped entirely if the trigger fires again for the same loan.
func (r *Run) Completed() {
	r.Status = StatusCompleted
	r.UpdatedAt = time.Now().UTC()
}

// Failed records a step's error without discarding the run — AttemptCount
// lets the caller decide whether to retry, and LastError is kept for
// operator visibility (ADR-008's "Saga state").
func (r *Run) Failed(err error) {
	r.Status = StatusFailed
	r.AttemptCount++
	if err != nil {
		r.LastError = err.Error()
	}
	r.UpdatedAt = time.Now().UTC()
}
