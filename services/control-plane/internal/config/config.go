package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	BunExecutorURL    string
	PythonExecutorURL string
	Port              string
	RedisURL          string // default: "localhost:6379"
	WorkerCount       int    // default: 5
}

// Load reads configuration from environment variables, falling back to sensible
// defaults for local development.
func Load() Config {
	workerCount := 5
	if v := os.Getenv("WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		}
	}

	return Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://runeforge:runeforge@localhost:5432/runeforge"),
		BunExecutorURL:    getEnv("BUN_EXECUTOR_URL", "http://localhost:8081"),
		PythonExecutorURL: getEnv("PYTHON_EXECUTOR_URL", "http://localhost:8082"),
		Port:              getEnv("PORT", "8080"),
		RedisURL:          getEnv("REDIS_URL", "localhost:6379"),
		WorkerCount:       workerCount,
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
