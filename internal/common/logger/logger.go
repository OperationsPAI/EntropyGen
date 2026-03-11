package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a structured logger.
// service: the service name (e.g., "operator", "gateway")
// version: the service version (e.g., "0.1.0")
// isDev: true for console format (development), false for JSON (production)
func New(service, version string, isDev bool) (*zap.Logger, error) {
	var cfg zap.Config
	if isDev {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	base, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return base.With(
		zap.String("service", service),
		zap.String("version", version),
	), nil
}

// Must creates a logger or panics on error. Useful in main().
func Must(service, version string, isDev bool) *zap.Logger {
	l, err := New(service, version, isDev)
	if err != nil {
		panic(fmt.Sprintf("logger init failed: %v", err))
	}
	return l
}
