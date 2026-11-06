package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config хранит настройки сервиса.
type Config struct {
	HttpAddr                string
	HttpReadHeaderTimeout   time.Duration
	HttpReadTimeout         time.Duration
	HttpWriteTimeout        time.Duration
	HttpIdleTimeout         time.Duration
	LogLevel                string
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	DatabasePingTimeout     time.Duration
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

	shutdownTimeout, err := parseDurationEnvVar("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, fmt.Errorf("invalid parse duration: %w", err)
	}

	httpReadHeaderTimeout, err := parseDurationEnvVar("HTTP_READ_HEADER_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	httpReadTimeout, err := parseDurationEnvVar("HTTP_READ_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	httpWriteTimeout, err := parseDurationEnvVar("HTTP_WRITE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	httpIdleTimeout, err := parseDurationEnvVar("HTTP_IDLE_TIMEOUT")
	if err != nil {
		return Config{}, err
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
	if databaseMaxConns <= 0 {
		return Config{}, fmt.Errorf("DATABASE_MAX_CONNS must be greater than zero")
	}
	if databaseMinConns < 0 || databaseMinConns > databaseMaxConns {
		return Config{}, fmt.Errorf("DATABASE_MIN_CONNS must be between zero and DATABASE_MAX_CONNS")
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
	databasePingTimeout, err := parseDurationEnvVar("DATABASE_PING_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HttpAddr:                httpAddr,
		HttpReadHeaderTimeout:   httpReadHeaderTimeout,
		HttpReadTimeout:         httpReadTimeout,
		HttpWriteTimeout:        httpWriteTimeout,
		HttpIdleTimeout:         httpIdleTimeout,
		LogLevel:                logLevel,
		ShutdownTimeout:         shutdownTimeout,
		DatabaseURL:             databaseURL,
		DatabaseMaxConns:        databaseMaxConns,
		DatabaseMinConns:        databaseMinConns,
		DatabaseMaxConnLifetime: databaseMaxConnsLifetime,
		DatabaseConnectTimeout:  databaseConnectTimeout,
		DatabaseQueryTimeout:    databaseQueryTimeout,
		DatabasePingTimeout:     databasePingTimeout,
	}

	return cfg, nil
}

func parseDurationEnvVar(envVar string) (time.Duration, error) {
	durationStr := os.Getenv(envVar)
	if durationStr == "" {
		return 0, fmt.Errorf("%s is required", envVar)
	}

	return parsePositiveDuration(envVar, durationStr)
}

func parsePositiveDuration(envVar, durationStr string) (time.Duration, error) {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %v", envVar, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", envVar)
	}

	return duration, nil
}

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
