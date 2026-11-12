// Package httpclient implements saga.CirculationClient and
// saga.MembershipClient over plain net/http — the same inter-service
// communication tool every other service in this system uses (no gRPC, no
// broker, AT-002). Reuses the internal-service-JWT pattern from
// lms-circulation-api's own membershipclient/catalogclient adapters.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/code-corhuila/lms-workflow/internal/domain/saga"
)

type CirculationClient struct {
	baseURL   string
	jwtSecret string
	http      *http.Client
}

func NewCirculationClient(baseURL, jwtSecret string) *CirculationClient {
	return &CirculationClient{baseURL: baseURL, jwtSecret: jwtSecret, http: &http.Client{Timeout: 5 * time.Second}}
}

// overdueLoanResponse mirrors lms-circulation-api's existing loanResponse
// shape (internal/adapter/in/httpapi/handler/loan_handler.go) — `id`, not
// `loanId`; only the fields this saga actually needs.
type overdueLoanResponse struct {
	ID        string `json:"id"`
	StudentID string `json:"studentId"`
}

type overdueLoansEnvelope struct {
	Data []overdueLoanResponse `json:"data"`
}

// FindOverdueLoans calls lms-circulation-api's existing GET /api/v1/loans/overdue
// (HU-08) — the same query lms-worker's ticker independently uses to send
// due-date notices; this saga is the one that acts on what it finds.
//
// NOTE: pagination (page/limit query params, meta.totalPages in the
// response) is not yet handled here — the real deployment needs this once
// the overdue backlog can exceed one page; out of scope for this repo's
// first cut.
func (c *CirculationClient) FindOverdueLoans(ctx context.Context) ([]saga.OverdueLoan, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/loans/overdue")
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling circulation-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("circulation-service returned %d", resp.StatusCode)
	}

	var body overdueLoansEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding circulation-service response: %w", err)
	}

	loans := make([]saga.OverdueLoan, 0, len(body.Data))
	for _, l := range body.Data {
		loans = append(loans, saga.OverdueLoan{LoanID: l.ID, StudentID: l.StudentID})
	}
	return loans, nil
}

// MarkLate is step one of the saga (ADR-008).
func (c *CirculationClient) MarkLate(ctx context.Context, loanID string) error {
	req, err := c.newRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/loans/%s/mark-late", loanID))
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling circulation-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("circulation-service returned %d", resp.StatusCode)
	}
	return nil
}

func (c *CirculationClient) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	token, err := mintServiceToken(c.jwtSecret, "lms-workflow")
	if err != nil {
		return nil, fmt.Errorf("minting internal service token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}
