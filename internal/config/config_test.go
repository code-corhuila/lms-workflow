package config

import (
	"testing"
	"time"
)

// setEnv sets every variable Load reads, so a test never picks up the
// developer's own environment.
func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	vars := map[string]string{
		"DB_HOST": "", "DB_PORT": "", "DB_USER": "", "DB_PASSWORD": "", "DB_NAME": "",
		"INTERNAL_JWT_SECRET": "secret", "CIRCULATION_SERVICE_URL": "", "MEMBERSHIP_SERVICE_URL": "",
		"POLL_INTERVAL": "", "LOG_LEVEL": "",
	}
	for k, v := range overrides {
		vars[k] = v
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.PollInterval != 15*time.Minute {
		t.Errorf("PollInterval = %v, want 15m", cfg.PollInterval)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.CirculationServiceURL != "http://circulation-service:8080" {
		t.Errorf("CirculationServiceURL = %q", cfg.CirculationServiceURL)
	}
	if got, want := cfg.DSN(), "postgres://lms_user:lms_password@localhost:5432/lms_db?sslmode=disable"; got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	setEnv(t, map[string]string{
		"DB_HOST": "db", "DB_PORT": "6543", "DB_USER": "u", "DB_PASSWORD": "p", "DB_NAME": "n",
		"MEMBERSHIP_SERVICE_URL": "http://membership:9000", "POLL_INTERVAL": "30s", "LOG_LEVEL": "debug",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.DSN(), "postgres://u:p@db:6543/n?sslmode=disable"; got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
	if cfg.MembershipServiceURL != "http://membership:9000" {
		t.Errorf("MembershipServiceURL = %q", cfg.MembershipServiceURL)
	}
	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadRequiresInternalJWTSecret(t *testing.T) {
	setEnv(t, map[string]string{"INTERNAL_JWT_SECRET": ""})

	if _, err := Load(); err == nil {
		t.Error("Load() error = nil, want an error when INTERNAL_JWT_SECRET is empty")
	}
}

func TestLoadRejectsInvalidPollInterval(t *testing.T) {
	setEnv(t, map[string]string{"POLL_INTERVAL": "every-hour"})

	if _, err := Load(); err == nil {
		t.Error("Load() error = nil, want an error for an invalid POLL_INTERVAL")
	}
}
