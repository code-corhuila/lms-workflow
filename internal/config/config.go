// Package config loads configuration from environment variables — the only source of
// environment-specific configuration (library-docs/05-architecture/cross-cutting.md).
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// DBHost/DBPort/DBUser/DBPassword/DBName point at the shared PostgreSQL
	// instance's `workflow` schema (Anexo J) — this repo's own migrations/
	// folder owns that schema; see README's "Migration scope" for why there
	// is no separate lms-workflow-db repo.
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// InternalJWTSecret mints the short-lived service-to-service token this
	// saga uses to call lms-circulation-api and lms-membership-api —
	// deliberately separate from any end-user RS256 session token
	// (rules/2-anexos/C-api-hexagonal.md, numeral 5.3.7).
	InternalJWTSecret string

	CirculationServiceURL string
	MembershipServiceURL  string

	// PollInterval drives how often this saga checks lms-circulation-api for
	// newly-overdue loans — reuses ADR-009's 15-minute cadence for
	// consistency, since nothing about this saga needs a different one.
	PollInterval time.Duration

	LogLevel string
}

func Load() (*Config, error) {
	pollInterval, err := time.ParseDuration(getEnv("POLL_INTERVAL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
	}

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "lms_user"),
		DBPassword: getEnv("DB_PASSWORD", "lms_password"),
		DBName:     getEnv("DB_NAME", "lms_db"),

		InternalJWTSecret: getEnv("INTERNAL_JWT_SECRET", ""),

		CirculationServiceURL: getEnv("CIRCULATION_SERVICE_URL", "http://circulation-service:8080"),
		MembershipServiceURL:  getEnv("MEMBERSHIP_SERVICE_URL", "http://membership-service:8080"),

		PollInterval: pollInterval,

		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	if cfg.InternalJWTSecret == "" {
		return nil, fmt.Errorf("INTERNAL_JWT_SECRET must be set")
	}

	return cfg, nil
}

// DSN builds the PostgreSQL connection string (pgx).
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
