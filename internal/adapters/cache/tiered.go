package cache

import (
	"context"

	"github.com/devchuckcamp/commit-coach/internal/observability"
	"github.com/devchuckcamp/commit-coach/internal/ports"
)

// TieredCache combines an in-memory L1 cache with a disk-based L2 cache.
// Reads check L1 first, then L2. Writes go to both.
// If disk cache fails, it degrades gracefully to memory-only.
type TieredCache struct {
	memory   *InMemory
	disk     *DiskCache
	degraded bool // true if disk cache failed
}

// NewTieredCache creates a tiered cache with memory L1 and disk L2.
// If diskDir is empty, uses DefaultCacheDir().
// If maxDiskEntries is 0, defaults to 100.
// Returns a memory-only cache if disk initialization fails.
func NewTieredCache(diskDir string, maxDiskEntries int) *TieredCache {
	memory := NewInMemory()

	disk, err := NewDiskCache(diskDir, maxDiskEntries)
	if err != nil {
		observability.Logger().Printf("cache: disk cache unavailable, using memory-only: %v", err)
		return &TieredCache{
			memory:   memory,
			disk:     nil,
			degraded: true,
		}
	}

	return &TieredCache{
		memory:   memory,
		disk:     disk,
		degraded: false,
	}
}

// Get retrieves cached suggestions, checking L1 (memory) first, then L2 (disk).
func (c *TieredCache) Get(ctx context.Context, key string) ([]ports.CommitSuggestion, error) {
	// Try L1 (memory) first
	if suggestions, err := c.memory.Get(ctx, key); err == nil {
		return suggestions, nil
	}

	// Try L2 (disk) if available
	if c.disk != nil && !c.degraded {
		suggestions, err := c.disk.Get(ctx, key)
		if err == nil {
			// Promote to L1
			_ = c.memory.Set(ctx, key, suggestions)
			return suggestions, nil
		}
	}

	return nil, cacheError("cache miss")
}

// Set stores suggestions in both L1 and L2 caches.
func (c *TieredCache) Set(ctx context.Context, key string, suggestions []ports.CommitSuggestion) error {
	// Always set in L1
	if err := c.memory.Set(ctx, key, suggestions); err != nil {
		return err
	}

	// Try L2 if available
	if c.disk != nil && !c.degraded {
		if err := c.disk.Set(ctx, key, suggestions); err != nil {
			// Log warning but don't fail
			observability.Logger().Printf("cache: disk write failed: %v", err)
		}
	}

	return nil
}

// Clear removes all entries from both caches.
func (c *TieredCache) Clear(ctx context.Context) error {
	_ = c.memory.Clear(ctx)

	if c.disk != nil && !c.degraded {
		if err := c.disk.Clear(ctx); err != nil {
			observability.Logger().Printf("cache: disk clear failed: %v", err)
		}
	}

	return nil
}

// Size returns the number of entries in L1 (memory) cache.
// This is the "hot" cache size.
func (c *TieredCache) Size(ctx context.Context) (int, error) {
	return c.memory.Size(ctx)
}

// DiskSize returns the number of entries in L2 (disk) cache.
// Returns 0 if disk cache is unavailable.
func (c *TieredCache) DiskSize(ctx context.Context) (int, error) {
	if c.disk == nil || c.degraded {
		return 0, nil
	}
	return c.disk.Size(ctx)
}

// IsDegraded returns true if operating in memory-only mode due to disk errors.
func (c *TieredCache) IsDegraded() bool {
	return c.degraded
}

// cacheError is a simple error type for cache misses.
type cacheError string

func (e cacheError) Error() string {
	return string(e)
}
