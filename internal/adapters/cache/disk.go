package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/devchuckcamp/commit-coach/internal/observability"
	"github.com/devchuckcamp/commit-coach/internal/ports"
)

// DiskCache implements ports.Cache with file-based JSON storage.
// It uses LRU eviction when the cache exceeds maxEntries.
type DiskCache struct {
	mu         sync.RWMutex
	dir        string
	maxEntries int
	metadata   *cacheMetadata
}

// cacheMetadata tracks cache entries for LRU eviction.
type cacheMetadata struct {
	Entries map[string]cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Key       string    `json:"key"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
	AccessedAt time.Time `json:"accessed_at"`
}

// diskEntry is the format stored on disk for each cache entry.
type diskEntry struct {
	Key         string                   `json:"key"`
	Suggestions []ports.CommitSuggestion `json:"suggestions"`
	CreatedAt   time.Time                `json:"created_at"`
}

// NewDiskCache creates a new disk-backed cache.
// If dir is empty, it uses DefaultCacheDir().
// maxEntries specifies the LRU eviction threshold (default 100 if 0).
func NewDiskCache(dir string, maxEntries int) (*DiskCache, error) {
	if dir == "" {
		var err error
		dir, err = DefaultCacheDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine cache directory: %w", err)
		}
	}

	if maxEntries <= 0 {
		maxEntries = 100
	}

	// Ensure directories exist
	entriesDir := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entriesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &DiskCache{
		dir:        dir,
		maxEntries: maxEntries,
	}

	// Load or create metadata
	if err := cache.loadMetadata(); err != nil {
		// Log warning but continue with empty metadata
		observability.Logger().Printf("cache: failed to load metadata, starting fresh: %v", err)
		cache.metadata = &cacheMetadata{
			Entries: make(map[string]cacheEntry),
		}
	}

	return cache, nil
}

func (c *DiskCache) metadataPath() string {
	return filepath.Join(c.dir, "metadata.json")
}

func (c *DiskCache) entryPath(filename string) string {
	return filepath.Join(c.dir, "entries", filename)
}

func (c *DiskCache) loadMetadata() error {
	data, err := os.ReadFile(c.metadataPath())
	if err != nil {
		if os.IsNotExist(err) {
			c.metadata = &cacheMetadata{
				Entries: make(map[string]cacheEntry),
			}
			return nil
		}
		return err
	}

	c.metadata = &cacheMetadata{}
	if err := json.Unmarshal(data, c.metadata); err != nil {
		return err
	}

	if c.metadata.Entries == nil {
		c.metadata.Entries = make(map[string]cacheEntry)
	}

	return nil
}

func (c *DiskCache) saveMetadata() error {
	data, err := json.MarshalIndent(c.metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.metadataPath(), data, 0644)
}

func keyToFilename(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:16]) + ".json"
}

// Get retrieves cached suggestions by key.
func (c *DiskCache) Get(ctx context.Context, key string) ([]ports.CommitSuggestion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.metadata.Entries[key]
	if !ok {
		return nil, fmt.Errorf("cache miss")
	}

	// Read entry file
	data, err := os.ReadFile(c.entryPath(entry.Filename))
	if err != nil {
		if os.IsNotExist(err) {
			// Entry file missing, remove from metadata
			delete(c.metadata.Entries, key)
			_ = c.saveMetadata()
			return nil, fmt.Errorf("cache miss")
		}
		return nil, fmt.Errorf("failed to read cache entry: %w", err)
	}

	var diskEnt diskEntry
	if err := json.Unmarshal(data, &diskEnt); err != nil {
		// Corrupt entry, remove it
		delete(c.metadata.Entries, key)
		_ = os.Remove(c.entryPath(entry.Filename))
		_ = c.saveMetadata()
		return nil, fmt.Errorf("cache miss")
	}

	// Update access time
	entry.AccessedAt = time.Now()
	c.metadata.Entries[key] = entry
	_ = c.saveMetadata() // Best effort

	// Return a copy
	result := make([]ports.CommitSuggestion, len(diskEnt.Suggestions))
	copy(result, diskEnt.Suggestions)
	return result, nil
}

// Set stores suggestions in the cache.
func (c *DiskCache) Set(ctx context.Context, key string, suggestions []ports.CommitSuggestion) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if necessary
	if len(c.metadata.Entries) >= c.maxEntries {
		c.evictLRU()
	}

	filename := keyToFilename(key)
	now := time.Now()

	// Store a copy
	cached := make([]ports.CommitSuggestion, len(suggestions))
	copy(cached, suggestions)

	diskEnt := diskEntry{
		Key:         key,
		Suggestions: cached,
		CreatedAt:   now,
	}

	data, err := json.MarshalIndent(diskEnt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	if err := os.WriteFile(c.entryPath(filename), data, 0644); err != nil {
		return fmt.Errorf("failed to write cache entry: %w", err)
	}

	c.metadata.Entries[key] = cacheEntry{
		Key:        key,
		Filename:   filename,
		CreatedAt:  now,
		AccessedAt: now,
	}

	if err := c.saveMetadata(); err != nil {
		// Log warning but don't fail
		observability.Logger().Printf("cache: failed to save metadata: %v", err)
	}

	return nil
}

// evictLRU removes the least recently used entry.
// Must be called with mu held.
func (c *DiskCache) evictLRU() {
	if len(c.metadata.Entries) == 0 {
		return
	}

	// Find oldest entry by access time
	var oldest cacheEntry
	oldestKey := ""
	for key, entry := range c.metadata.Entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldest.AccessedAt) {
			oldest = entry
			oldestKey = key
		}
	}

	if oldestKey != "" {
		_ = os.Remove(c.entryPath(oldest.Filename))
		delete(c.metadata.Entries, oldestKey)
	}
}

// Clear removes all cache entries.
func (c *DiskCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all entry files
	for _, entry := range c.metadata.Entries {
		_ = os.Remove(c.entryPath(entry.Filename))
	}

	c.metadata.Entries = make(map[string]cacheEntry)
	return c.saveMetadata()
}

// Size returns the number of cached entries.
func (c *DiskCache) Size(ctx context.Context) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.metadata.Entries), nil
}

// Keys returns all cache keys, sorted by access time (most recent first).
func (c *DiskCache) Keys(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]cacheEntry, 0, len(c.metadata.Entries))
	for _, entry := range c.metadata.Entries {
		entries = append(entries, entry)
	}

	// Sort by access time, most recent first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AccessedAt.After(entries[j].AccessedAt)
	})

	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}

	return keys, nil
}
