package saga

import (
	"errors"
	"testing"
)

func TestNewRunStartsPending(t *testing.T) {
	run := NewRun("loan-1")

	if run.LoanID != "loan-1" {
		t.Errorf("LoanID = %q, want %q", run.LoanID, "loan-1")
	}
	if run.Status != StatusPending {
		t.Errorf("Status = %q, want %q", run.Status, StatusPending)
	}
	if run.AttemptCount != 0 {
		t.Errorf("AttemptCount = %d, want 0", run.AttemptCount)
	}
	if run.CreatedAt.IsZero() || !run.CreatedAt.Equal(run.UpdatedAt) {
		t.Errorf("CreatedAt/UpdatedAt = %v/%v, want equal non-zero timestamps", run.CreatedAt, run.UpdatedAt)
	}
}

func TestMarkedLateAdvancesStatus(t *testing.T) {
	run := NewRun("loan-1")
	before := run.UpdatedAt

	run.MarkedLate()

	if run.Status != StatusMarkedLate {
		t.Errorf("Status = %q, want %q", run.Status, StatusMarkedLate)
	}
	if run.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt moved backwards: %v < %v", run.UpdatedAt, before)
	}
}

func TestCompletedIsTerminal(t *testing.T) {
	run := NewRun("loan-1")
	run.MarkedLate()

	run.Completed()

	if run.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", run.Status, StatusCompleted)
	}
}

func TestFailedRecordsErrorAndCountsAttempts(t *testing.T) {
	run := NewRun("loan-1")

	run.Failed(errors.New("first"))
	run.Failed(errors.New("second"))

	if run.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", run.Status, StatusFailed)
	}
	if run.AttemptCount != 2 {
		t.Errorf("AttemptCount = %d, want 2", run.AttemptCount)
	}
	if run.LastError != "second" {
		t.Errorf("LastError = %q, want %q", run.LastError, "second")
	}
}

func TestRecordFailureKeepsStatus(t *testing.T) {
	run := NewRun("loan-1")
	run.MarkedLate()

	run.RecordFailure(errors.New("membership down"))

	if run.Status != StatusMarkedLate {
		t.Errorf("Status = %q, want %q", run.Status, StatusMarkedLate)
	}
	if run.AttemptCount != 1 || run.LastError != "membership down" {
		t.Errorf("AttemptCount/LastError = %d/%q, want 1/%q", run.AttemptCount, run.LastError, "membership down")
	}
}

func TestFailedWithNilErrorKeepsPreviousMessage(t *testing.T) {
	run := NewRun("loan-1")
	run.Failed(errors.New("boom"))

	run.Failed(nil)

	if run.LastError != "boom" {
		t.Errorf("LastError = %q, want %q", run.LastError, "boom")
	}
	if run.AttemptCount != 2 {
		t.Errorf("AttemptCount = %d, want 2", run.AttemptCount)
	}
}
