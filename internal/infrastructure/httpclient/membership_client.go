package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type MembershipClient struct {
	baseURL   string
	jwtSecret string
	http      *http.Client
}

func NewMembershipClient(baseURL, jwtSecret string) *MembershipClient {
	return &MembershipClient{baseURL: baseURL, jwtSecret: jwtSecret, http: &http.Client{Timeout: 5 * time.Second}}
}

// Suspend is step two of the saga (ADR-008) — applies the flat suspension
// via lms-membership-api's own endpoint, same one lms-circulation-api's
// direct (synchronous) flow already calls for a late return. This saga never
// decides *how* a suspension works, only *when* to trigger it.
func (c *MembershipClient) Suspend(ctx context.Context, studentID string) error {
	token, err := mintServiceToken(c.jwtSecret, "lms-workflow")
	if err != nil {
		return fmt.Errorf("minting internal service token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/students/%s/suspend", c.baseURL, studentID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling membership-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("membership-service returned %d", resp.StatusCode)
	}
	return nil
}
