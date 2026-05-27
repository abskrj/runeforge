package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"

	"go.uber.org/zap"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	BunExecutorURL    string
	PythonExecutorURL string
	Port              string
	RedisURL          string // default: "localhost:6379"
	WorkerCount       int    // default: 5
	EncryptionKey     string // hex-encoded 32-byte AES key; generated ephemerally if empty
	ClickHouseDSN     string // optional Phase 5 metrics store DSN
	LogsBucket        string // optional Phase 5 object storage bucket for logs
	ReplayBucket      string // optional Phase 5 object storage bucket for replay payloads
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
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
		ClickHouseDSN:     os.Getenv("CLICKHOUSE_DSN"),
		LogsBucket:        os.Getenv("LOGS_BUCKET"),
		ReplayBucket:      os.Getenv("REPLAY_BUCKET"),
	}
}

// EncryptionKeyBytes parses EncryptionKey as a 64-character hex string (32 bytes)
// or generates a random ephemeral key if ENCRYPTION_KEY is empty.
// Logs a warning when generating an ephemeral key — not suitable for production.
func (c *Config) EncryptionKeyBytes(log *zap.Logger) []byte {
	if c.EncryptionKey != "" {
		key, err := hex.DecodeString(c.EncryptionKey)
		if err == nil && len(key) == 32 {
			return key
		}
		log.Warn("ENCRYPTION_KEY is set but invalid (must be 64 hex chars); generating ephemeral key",
			zap.Int("got_bytes", len(key)),
			zap.Error(err),
		)
	} else {
		log.Warn("ENCRYPTION_KEY not set; generating a random ephemeral key — secrets will not survive restarts")
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// If we can't read random bytes, use a fixed fallback (still better than panic).
		log.Error("failed to generate random encryption key; using zeroed key", zap.Error(err))
		return make([]byte, 32)
	}
	return key
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
