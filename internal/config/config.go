package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DefaultInterval time.Duration
	DefaultWorkers  int

	PGHost     string
	PGPort     int
	PGUser     string
	PGPassword string
	PGDatabase string

	ControlAddr string
}

func Load() Config {
	interval := parseDurationEnv("CLI_APP_TIMER_INTERVAL", 3*time.Minute)
	workers := parseIntEnv("CLI_APP_WORKERS_COUNT", 3)
	pgPort := parseIntEnv("POSTGRES_PORT", 5432)
	return Config{
		DefaultInterval: interval,
		DefaultWorkers:  workers,
		PGHost:          getenv("POSTGRES_HOST", "localhost"),
		PGPort:          pgPort,
		PGUser:          getenv("POSTGRES_USER", "postgres"),
		PGPassword:      getenv("POSTGRES_PASSWORD", "changeme"),
		PGDatabase:      getenv("POSTGRES_DBNAME", "rsshub"),
		ControlAddr:     getenv("CONTROL_ADDR", "127.0.0.1:8088"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseIntEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
