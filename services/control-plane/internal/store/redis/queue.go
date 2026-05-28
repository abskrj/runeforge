package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const jobQueueKey = "velane:jobs"

// EgressPolicyJob carries egress policy in an enqueued job.
type EgressPolicyJob struct {
	BlockedCIDRs   []string `json:"blocked_cidrs"`
	BlockedDomains []string `json:"blocked_domains"`
}

// Job is the unit of async work pushed to Redis.
type Job struct {
	InvocationID  string            `json:"invocation_id"`
	SnippetID     string            `json:"snippet_id"`
	VersionID     string            `json:"version_id"`
	TenantID      string            `json:"tenant_id"`
	Language      string            `json:"language"`
	Code          string            `json:"code"`
	Input         string            `json:"input"`
	TimeoutMs     int               `json:"timeout_ms"`
	MaxMemoryMB   int               `json:"max_memory_mb"`
	CallbackURL   string            `json:"callback_url,omitempty"`
	Env           string            `json:"env"`
	SecretEnvVars map[string]string `json:"secret_env_vars,omitempty"`
	Libraries     map[string]string `json:"libraries,omitempty"`
	EgressPolicy  *EgressPolicyJob  `json:"egress_policy,omitempty"`
}

// Enqueue serialises job as JSON and pushes it to the left of the job list.
func (c *Client) Enqueue(ctx context.Context, job Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("enqueue marshal: %w", err)
	}
	if err := c.rdb.LPush(ctx, jobQueueKey, data).Err(); err != nil {
		return fmt.Errorf("enqueue lpush: %w", err)
	}
	return nil
}

// Dequeue blocks until a job is available (BRPOP with 0 timeout = block forever)
// or until the context is cancelled, whichever comes first.
// Returns (nil, nil) when the context is cancelled.
func (c *Client) Dequeue(ctx context.Context) (*Job, error) {
	// BRPOP returns a slice of [key, value].
	result, err := c.rdb.BRPop(ctx, 0, jobQueueKey).Result()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}
		// redis.Nil shouldn't happen with timeout=0 but handle gracefully.
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("dequeue brpop: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("dequeue: unexpected brpop result length %d", len(result))
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("dequeue unmarshal: %w", err)
	}
	return &job, nil
}
