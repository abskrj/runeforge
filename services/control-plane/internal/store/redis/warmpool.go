package redisstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const poolKeyPrefix = "velane:pool:"

func poolKey(snippetID, env string) string {
	return poolKeyPrefix + snippetID + ":" + env
}

// InitPool ensures that the warm pool for a snippet+env has at least minInstances
// slot tokens. It is idempotent: if the pool already has >= minInstances slots it
// does nothing.
func (c *Client) InitPool(ctx context.Context, snippetID, env string, minInstances int) error {
	key := poolKey(snippetID, env)

	current, err := c.rdb.LLen(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("initpool llen: %w", err)
	}

	needed := int64(minInstances) - current
	if needed <= 0 {
		return nil
	}

	slots := make([]any, needed)
	for i := range slots {
		slots[i] = uuid.New().String()
	}

	if err := c.rdb.LPush(ctx, key, slots...).Err(); err != nil {
		return fmt.Errorf("initpool lpush: %w", err)
	}
	return nil
}

// ClaimSlot attempts to non-blockingly claim a slot from the warm pool.
// Returns ("", false) if no slot is available (pool empty / cold start).
func (c *Client) ClaimSlot(ctx context.Context, snippetID, env string) (slotID string, ok bool) {
	key := poolKey(snippetID, env)
	val, err := c.rdb.LPop(ctx, key).Result()
	if err != nil {
		// redis.Nil means empty list — normal, not an error condition.
		return "", false
	}
	return val, true
}

// ReleaseSlot returns a slot token to the warm pool.
func (c *Client) ReleaseSlot(ctx context.Context, snippetID, env, slotID string) error {
	key := poolKey(snippetID, env)
	if err := c.rdb.LPush(ctx, key, slotID).Err(); err != nil {
		return fmt.Errorf("releaseslot lpush: %w", err)
	}
	return nil
}

// PoolSize returns the current number of available slots.
func (c *Client) PoolSize(ctx context.Context, snippetID, env string) (int64, error) {
	key := poolKey(snippetID, env)
	n, err := c.rdb.LLen(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("poolsize llen: %w", err)
	}
	return n, nil
}
