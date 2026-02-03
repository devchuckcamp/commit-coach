package cache

import (
	"context"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
)

func TestTieredCache_GetSet(t *testing.T) {
	dir := t.TempDir()
	cache := NewTieredCache(dir, 100)
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "add feature"},
		{Type: "fix", Subject: "fix bug"},
	}

	// Test cache miss
	_, err := cache.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error on cache miss, got nil")
	}

	// Test set and get
	err = cache.Set(ctx, "key1", suggestions)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(got) != len(suggestions) {
		t.Errorf("expected %d suggestions, got %d", len(suggestions), len(got))
	}
}

func TestTieredCache_L1Hit(t *testing.T) {
	dir := t.TempDir()
	cache := NewTieredCache(dir, 100)
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	// Set value
	cache.Set(ctx, "key1", suggestions)

	// Get should hit L1 (memory)
	got, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got[0].Subject != "test" {
		t.Errorf("expected subject 'test', got %s", got[0].Subject)
	}

	// Verify L1 has the entry
	size, _ := cache.Size(ctx)
	if size != 1 {
		t.Errorf("expected L1 size 1, got %d", size)
	}
}

func TestTieredCache_L2Promotion(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "disk"}}

	// Create first cache and set value
	cache1 := NewTieredCache(dir, 100)
	cache1.Set(ctx, "key1", suggestions)

	// Create new cache (L1 is empty, L2 has data)
	cache2 := NewTieredCache(dir, 100)

	// L1 should be empty
	size, _ := cache2.Size(ctx)
	if size != 0 {
		t.Errorf("expected fresh L1 to be empty, got size %d", size)
	}

	// Get should hit L2 and promote to L1
	got, err := cache2.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got[0].Subject != "disk" {
		t.Errorf("expected subject 'disk', got %s", got[0].Subject)
	}

	// L1 should now have the entry
	size, _ = cache2.Size(ctx)
	if size != 1 {
		t.Errorf("expected L1 size 1 after promotion, got %d", size)
	}
}

func TestTieredCache_Clear(t *testing.T) {
	dir := t.TempDir()
	cache := NewTieredCache(dir, 100)
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	cache.Set(ctx, "key1", suggestions)
	cache.Set(ctx, "key2", suggestions)

	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// L1 should be empty
	size, _ := cache.Size(ctx)
	if size != 0 {
		t.Errorf("expected L1 size 0 after clear, got %d", size)
	}

	// L2 should be empty
	diskSize, _ := cache.DiskSize(ctx)
	if diskSize != 0 {
		t.Errorf("expected L2 size 0 after clear, got %d", diskSize)
	}
}

func TestTieredCache_DiskSize(t *testing.T) {
	dir := t.TempDir()
	cache := NewTieredCache(dir, 100)
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	// Initially empty
	diskSize, _ := cache.DiskSize(ctx)
	if diskSize != 0 {
		t.Errorf("expected disk size 0, got %d", diskSize)
	}

	cache.Set(ctx, "key1", suggestions)

	diskSize, _ = cache.DiskSize(ctx)
	if diskSize != 1 {
		t.Errorf("expected disk size 1, got %d", diskSize)
	}
}

func TestTieredCache_Degraded(t *testing.T) {
	// Create cache with invalid directory (should degrade)
	cache := NewTieredCache("/nonexistent/path/that/cannot/exist", 100)

	if !cache.IsDegraded() {
		// On some systems this might succeed, so just check it doesn't crash
		t.Log("cache not degraded (directory might be writable)")
	}

	// Should still work in memory-only mode
	ctx := context.Background()
	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	err := cache.Set(ctx, "key1", suggestions)
	if err != nil {
		t.Fatalf("Set should work in degraded mode: %v", err)
	}

	got, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get should work in degraded mode: %v", err)
	}

	if got[0].Subject != "test" {
		t.Error("should get correct value in degraded mode")
	}
}

func TestTieredCache_DegradedDiskSize(t *testing.T) {
	// Create degraded cache
	cache := &TieredCache{
		memory:   NewInMemory(),
		disk:     nil,
		degraded: true,
	}

	ctx := context.Background()
	diskSize, err := cache.DiskSize(ctx)
	if err != nil {
		t.Fatalf("DiskSize should not error: %v", err)
	}

	if diskSize != 0 {
		t.Errorf("degraded cache should report disk size 0, got %d", diskSize)
	}
}
