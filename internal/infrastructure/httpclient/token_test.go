package httpclient

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-internal-secret"

// parseServiceToken verifies a token the way a receiving service would:
// HS256 only, signed with the shared secret, not expired.
func parseServiceToken(t *testing.T, raw string) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(testSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		t.Fatalf("token does not verify: %v", err)
	}
	return claims
}

func TestMintServiceTokenIsShortLivedHS256(t *testing.T) {
	raw, err := mintServiceToken(testSecret, "lms-workflow")
	if err != nil {
		t.Fatalf("mintServiceToken() error = %v", err)
	}

	claims := parseServiceToken(t, raw)

	if sub, _ := claims.GetSubject(); sub != "lms-workflow" {
		t.Errorf("sub = %q, want %q", sub, "lms-workflow")
	}
	iat, _ := claims.GetIssuedAt()
	exp, _ := claims.GetExpirationTime()
	if iat == nil || exp == nil {
		t.Fatalf("iat/exp missing: %v", claims)
	}
	if ttl := exp.Sub(iat.Time); ttl <= 0 || ttl > time.Minute {
		t.Errorf("token TTL = %v, want (0, 1m]", ttl)
	}
}

func TestMintServiceTokenRejectedWithWrongSecret(t *testing.T) {
	raw, err := mintServiceToken(testSecret, "lms-workflow")
	if err != nil {
		t.Fatalf("mintServiceToken() error = %v", err)
	}

	_, err = jwt.Parse(raw, func(*jwt.Token) (any, error) { return []byte("other-secret"), nil })
	if err == nil {
		t.Error("token verified with the wrong secret")
	}
}
