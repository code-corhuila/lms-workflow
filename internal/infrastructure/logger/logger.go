// Package logger configures the shared structured logger — see
// library-docs/05-architecture/cross-cutting.md ("Logging").
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a JSON zap.Logger at the given level ("debug", "info", "warn", "error").
func New(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return cfg.Build()
}
