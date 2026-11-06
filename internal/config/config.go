package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config хранит настройки сервиса.
type Config struct {
	HttpAddr                 string
	LogLevel                 string
	DatabaseURL              string
	DatabaseMaxConns         int32
	DatabaseMinConns         int32
	DatabaseMaxConnLifetime  time.Duration
	DatabaseConnectTimeout   time.Duration
	DatabaseQueryTimeout     time.Duration
	DatabaseIdempotencyTTL   time.Duration
	ShutdownTimeout          time.Duration
	HTTPReadHeaderTimeout    time.Duration
	HTTPReadTimeout          time.Duration
	HTTPWriteTimeout         time.Duration
	HTTPIdleTimeout          time.Duration
	TelemetryShutdownTimeout time.Duration
}

// LoadConfig загружает из .env конфигурацию и валидирует.
func LoadAndValidateConfig() (Config, error) {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		return Config{}, fmt.Errorf("LOG_LEVEL is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	databaseMaxConns, err := parseIntEnvVar("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse int: %w", err)
	}

	databaseMinConns, err := parseIntEnvVar("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse int: %w", err)
	}

	databaseMaxConnsLifetime, err := parseDurationEnvVar("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	databaseConnectTimeout, err := parseDurationEnvVar("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	databaseQueryTimeout, err := parseDurationEnvVar("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	databaseIdempotencyTTL, err := parseDurationEnvVar("DATABASE_IDEMPOTENCY_TTL")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	shutdownTimeout, err := parseDurationEnvVar("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	readHeaderTimeout, err := parseDurationEnvVar("HTTP_READ_HEADER_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	readTimeout, err := parseDurationEnvVar("HTTP_READ_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	writeTimeout, err := parseDurationEnvVar("HTTP_WRITE_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	idleTimeout, err := parseDurationEnvVar("HTTP_IDLE_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	telemetryShutdownTimeout, err := parseDurationEnvVar("TELEMETRY_SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	cfg := Config{
		HttpAddr:                 httpAddr,
		LogLevel:                 logLevel,
		DatabaseURL:              databaseURL,
		DatabaseMaxConns:         databaseMaxConns,
		DatabaseMinConns:         databaseMinConns,
		DatabaseMaxConnLifetime:  databaseMaxConnsLifetime,
		DatabaseConnectTimeout:   databaseConnectTimeout,
		DatabaseQueryTimeout:     databaseQueryTimeout,
		DatabaseIdempotencyTTL:   databaseIdempotencyTTL,
		ShutdownTimeout:          shutdownTimeout,
		HTTPReadHeaderTimeout:    readHeaderTimeout,
		HTTPReadTimeout:          readTimeout,
		HTTPWriteTimeout:         writeTimeout,
		HTTPIdleTimeout:          idleTimeout,
		TelemetryShutdownTimeout: telemetryShutdownTimeout,
	}

	if cfg.DatabaseMaxConns <= 0 {
		return Config{}, fmt.Errorf("DATABASE_MAX_CONNS must be greater than zero")
	}
	if cfg.DatabaseMinConns < 0 || cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return Config{}, fmt.Errorf("DATABASE_MIN_CONNS must be between zero and DATABASE_MAX_CONNS")
	}

	for _, timeout := range []struct {
		name     string
		duration time.Duration
	}{
		{"DATABASE_MAX_CONN_LIFETIME", cfg.DatabaseMaxConnLifetime},
		{"DATABASE_CONNECT_TIMEOUT", cfg.DatabaseConnectTimeout},
		{"DATABASE_QUERY_TIMEOUT", cfg.DatabaseQueryTimeout},
		{"DATABASE_IDEMPOTENCY_TTL", cfg.DatabaseIdempotencyTTL},
		{"SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", cfg.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", cfg.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", cfg.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", cfg.HTTPIdleTimeout},
		{"TELEMETRY_SHUTDOWN_TIMEOUT", cfg.TelemetryShutdownTimeout},
	} {
		if timeout.duration <= 0 {
			return Config{}, fmt.Errorf("%s must be greater than zero", timeout.name)
		}
	}

	return cfg, nil
}

// parseDurationEnvVar парсит переменную окружения в time.Duration.
func parseDurationEnvVar(envVar string) (time.Duration, error) {
	durationStr := os.Getenv(envVar)
	if durationStr == "" {
		return 0, fmt.Errorf("%s is required", envVar)
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %v", envVar, err)
	}

	return duration, nil
}

// parseIntEnvVar парсит переменную окружения в int32.
func parseIntEnvVar(envVar string) (int32, error) {
	intStr := os.Getenv(envVar)
	if intStr == "" {
		return 0, fmt.Errorf("%s is required", envVar)
	}

	intValue, err := strconv.ParseInt(intStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %v", envVar, err)
	}

	return int32(intValue), nil
}
