package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

// requireServiceToken fails the test unless the request carries a valid
// internal service token for lms-workflow.
func requireServiceToken(t *testing.T, r *http.Request) {
	t.Helper()
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		t.Fatalf("Authorization header = %q, want a Bearer token", r.Header.Get("Authorization"))
	}
	if sub, _ := parseServiceToken(t, raw).GetSubject(); sub != "lms-workflow" {
		t.Errorf("token sub = %q, want %q", sub, "lms-workflow")
	}
}

func TestFindOverdueLoansMapsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/loans/overdue" {
			t.Errorf("request = %s %s, want GET /api/v1/loans/overdue", r.Method, r.URL.Path)
		}
		requireServiceToken(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"loan-1","studentId":"student-1","bookId":"ignored"},
			{"id":"loan-2","studentId":"student-2"}
		],"meta":{"totalPages":1}}`))
	}))
	defer srv.Close()

	loans, err := NewCirculationClient(srv.URL, testSecret).FindOverdueLoans(context.Background())
	if err != nil {
		t.Fatalf("FindOverdueLoans() error = %v", err)
	}

	want := []saga.OverdueLoan{{LoanID: "loan-1", StudentID: "student-1"}, {LoanID: "loan-2", StudentID: "student-2"}}
	if len(loans) != len(want) {
		t.Fatalf("loans = %+v, want %+v", loans, want)
	}
	for i := range want {
		if loans[i] != want[i] {
			t.Errorf("loans[%d] = %+v, want %+v", i, loans[i], want[i])
		}
	}
}

func TestFindOverdueLoansEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	loans, err := NewCirculationClient(srv.URL, testSecret).FindOverdueLoans(context.Background())
	if err != nil {
		t.Fatalf("FindOverdueLoans() error = %v", err)
	}
	if len(loans) != 0 {
		t.Errorf("loans = %+v, want none", loans)
	}
}

func TestFindOverdueLoansErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"non-200 status", http.StatusInternalServerError, `{"error":"boom"}`},
		{"malformed body", http.StatusOK, `not json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			if _, err := NewCirculationClient(srv.URL, testSecret).FindOverdueLoans(context.Background()); err == nil {
				t.Error("FindOverdueLoans() error = nil, want an error")
			}
		})
	}
}

func TestMarkLatePostsToLoanEndpoint(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/loans/loan-1/mark-late" {
			t.Errorf("request = %s %s, want POST /api/v1/loans/loan-1/mark-late", r.Method, r.URL.Path)
		}
		requireServiceToken(t, r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewCirculationClient(srv.URL, testSecret).MarkLate(context.Background(), "loan-1"); err != nil {
		t.Fatalf("MarkLate() error = %v", err)
	}
	if !called {
		t.Error("server was never called")
	}
}

func TestMarkLateNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := NewCirculationClient(srv.URL, testSecret).MarkLate(context.Background(), "loan-1"); err == nil {
		t.Error("MarkLate() error = nil, want an error for 404")
	}
}

func TestCirculationClientUnreachableIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := NewCirculationClient(url, testSecret).FindOverdueLoans(context.Background()); err == nil {
		t.Error("FindOverdueLoans() error = nil, want a connection error")
	}
}
