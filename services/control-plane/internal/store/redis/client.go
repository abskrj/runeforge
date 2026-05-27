// Package redisstore provides Redis-backed job queue and warm pool primitives.
package redisstore

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis connection and exposes queue and warm-pool operations.
type Client struct {
	rdb *redis.Client
}

// New creates a Redis client, pings the server, and returns a ready Client.
func New(addr string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

// Close releases the underlying Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}
