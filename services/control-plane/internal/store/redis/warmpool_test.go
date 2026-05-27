package redisstore_test

import (
	"context"
	"testing"
)

func TestWarmPool_InitPoolAndClaimRelease(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	snippetID := "snip-wp-1"
	env := "prod"

	// InitPool with 3 slots.
	if err := client.InitPool(ctx, snippetID, env, 3); err != nil {
		t.Fatalf("InitPool: %v", err)
	}

	size, err := client.PoolSize(ctx, snippetID, env)
	if err != nil {
		t.Fatalf("PoolSize: %v", err)
	}
	if size < 3 {
		t.Errorf("PoolSize = %d; want >= 3", size)
	}

	// Claim a slot.
	slotID, ok := client.ClaimSlot(ctx, snippetID, env)
	if !ok {
		t.Fatal("ClaimSlot returned false; expected a slot to be available")
	}
	if slotID == "" {
		t.Error("ClaimSlot returned empty slot ID")
	}

	// Pool should be one smaller.
	newSize, _ := client.PoolSize(ctx, snippetID, env)
	if newSize != size-1 {
		t.Errorf("PoolSize after claim = %d; want %d", newSize, size-1)
	}

	// Release the slot back.
	if err := client.ReleaseSlot(ctx, snippetID, env, slotID); err != nil {
		t.Fatalf("ReleaseSlot: %v", err)
	}

	// Pool should be back to original size.
	finalSize, _ := client.PoolSize(ctx, snippetID, env)
	if finalSize != size {
		t.Errorf("PoolSize after release = %d; want %d", finalSize, size)
	}
}

func TestWarmPool_ClaimEmptyPool(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	// Use a unique snippet ID so pool is guaranteed empty.
	snippetID := "snip-wp-empty"
	env := "dev"

	// Do not init pool — it should be empty.
	slotID, ok := client.ClaimSlot(ctx, snippetID, env)
	if ok {
		t.Errorf("ClaimSlot on empty pool returned ok=true, slotID=%q", slotID)
	}
}

func TestWarmPool_InitPoolIdempotent(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	snippetID := "snip-wp-idem"
	env := "prod"

	// Init with 2.
	if err := client.InitPool(ctx, snippetID, env, 2); err != nil {
		t.Fatalf("InitPool first: %v", err)
	}
	size1, _ := client.PoolSize(ctx, snippetID, env)

	// Init again with 2 — should not add more slots (idempotent when >= min).
	if err := client.InitPool(ctx, snippetID, env, 2); err != nil {
		t.Fatalf("InitPool second: %v", err)
	}
	size2, _ := client.PoolSize(ctx, snippetID, env)

	if size2 != size1 {
		t.Errorf("PoolSize after second InitPool = %d; want %d (idempotent)", size2, size1)
	}
}

func TestWarmPool_InitPoolTopUp(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	snippetID := "snip-wp-topup"
	env := "dev"

	// Init with 1.
	if err := client.InitPool(ctx, snippetID, env, 1); err != nil {
		t.Fatalf("InitPool initial: %v", err)
	}

	// Claim 1 so pool is empty.
	_, ok := client.ClaimSlot(ctx, snippetID, env)
	if !ok {
		t.Fatal("expected to claim a slot")
	}

	// InitPool with 2 should add 2 new slots.
	if err := client.InitPool(ctx, snippetID, env, 2); err != nil {
		t.Fatalf("InitPool top-up: %v", err)
	}

	size, _ := client.PoolSize(ctx, snippetID, env)
	if size < 2 {
		t.Errorf("PoolSize after top-up = %d; want >= 2", size)
	}
}
