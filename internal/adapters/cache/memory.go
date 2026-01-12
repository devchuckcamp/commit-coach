package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/chuckie/commit-coach/internal/ports"
)

// InMemory is a simple in-memory cache protected by a mutex.
type InMemory struct {
	mu    sync.RWMutex
	cache map[string][]ports.CommitSuggestion
}

// NewInMemory creates a new in-memory cache.
func NewInMemory() *InMemory {
	return &InMemory{
		cache: make(map[string][]ports.CommitSuggestion),
	}
}

// Get retrieves cached suggestions by key.
func (c *InMemory) Get(ctx context.Context, key string) ([]ports.CommitSuggestion, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		// Return a copy to prevent external mutation
		result := make([]ports.CommitSuggestion, len(val))
		copy(result, val)
		return result, nil
	}

	return nil, fmt.Errorf("cache miss")
}

// Set stores suggestions in the cache by key.
func (c *InMemory) Set(ctx context.Context, key string, suggestions []ports.CommitSuggestion) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy to prevent external mutation
	cached := make([]ports.CommitSuggestion, len(suggestions))
	copy(cached, suggestions)
	c.cache[key] = cached

	return nil
}

// Clear empties the cache.
func (c *InMemory) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]ports.CommitSuggestion)
}

// Size returns the number of cached entries.
func (c *InMemory) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
