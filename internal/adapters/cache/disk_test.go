package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
)

func TestDiskCache_GetSet(t *testing.T) {
	// Create temp directory for test
	dir := t.TempDir()
	cache, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	ctx := context.Background()
	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "add feature"},
		{Type: "fix", Subject: "fix bug"},
		{Type: "docs", Subject: "update docs"},
	}

	// Test cache miss
	_, err = cache.Get(ctx, "nonexistent")
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

	for i, s := range got {
		if s.Type != suggestions[i].Type || s.Subject != suggestions[i].Subject {
			t.Errorf("suggestion %d mismatch: got %+v, want %+v", i, s, suggestions[i])
		}
	}
}

func TestDiskCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "persistent"},
	}

	// Create cache and set value
	cache1, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	err = cache1.Set(ctx, "key1", suggestions)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Create new cache instance with same directory
	cache2, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Should be able to retrieve value
	got, err := cache2.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get from new cache failed: %v", err)
	}

	if len(got) != 1 || got[0].Subject != "persistent" {
		t.Errorf("expected persistent value, got %+v", got)
	}
}

func TestDiskCache_LRUEviction(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create cache with max 3 entries
	cache, err := NewDiskCache(dir, 3)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	// Add 3 entries
	cache.Set(ctx, "key1", suggestions)
	cache.Set(ctx, "key2", suggestions)
	cache.Set(ctx, "key3", suggestions)

	size, _ := cache.Size(ctx)
	if size != 3 {
		t.Errorf("expected size 3, got %d", size)
	}

	// Access key1 to make it most recently used
	cache.Get(ctx, "key1")

	// Add 4th entry - should evict key2 (oldest accessed)
	cache.Set(ctx, "key4", suggestions)

	size, _ = cache.Size(ctx)
	if size != 3 {
		t.Errorf("expected size 3 after eviction, got %d", size)
	}

	// key1 should still exist (was accessed)
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Error("key1 should still exist after eviction")
	}

	// key4 should exist (just added)
	_, err = cache.Get(ctx, "key4")
	if err != nil {
		t.Error("key4 should exist")
	}
}

func TestDiskCache_Clear(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	cache, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	cache.Set(ctx, "key1", suggestions)
	cache.Set(ctx, "key2", suggestions)

	size, _ := cache.Size(ctx)
	if size != 2 {
		t.Errorf("expected size 2, got %d", size)
	}

	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	size, _ = cache.Size(ctx)
	if size != 0 {
		t.Errorf("expected size 0 after clear, got %d", size)
	}

	// Verify files are removed
	entries, _ := os.ReadDir(filepath.Join(dir, "entries"))
	if len(entries) != 0 {
		t.Errorf("expected empty entries dir after clear, got %d files", len(entries))
	}
}

func TestDiskCache_Keys(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	cache, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}

	cache.Set(ctx, "key1", suggestions)
	cache.Set(ctx, "key2", suggestions)
	cache.Set(ctx, "key3", suggestions)

	// Access key1 to make it most recent
	cache.Get(ctx, "key1")

	keys, err := cache.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// First key should be most recently accessed (key1)
	if keys[0] != "key1" {
		t.Errorf("expected key1 to be most recent, got %s", keys[0])
	}
}

func TestDiskCache_CorruptEntry(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	cache, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}
	cache.Set(ctx, "key1", suggestions)

	// Corrupt the entry file
	filename := keyToFilename("key1")
	err = os.WriteFile(filepath.Join(dir, "entries", filename), []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}

	// Get should return cache miss and clean up
	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("expected error for corrupt entry")
	}

	size, _ := cache.Size(ctx)
	if size != 0 {
		t.Errorf("corrupt entry should have been removed, got size %d", size)
	}
}

func TestDiskCache_MissingEntryFile(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	cache, err := NewDiskCache(dir, 100)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	suggestions := []ports.CommitSuggestion{{Type: "feat", Subject: "test"}}
	cache.Set(ctx, "key1", suggestions)

	// Remove the entry file
	filename := keyToFilename("key1")
	os.Remove(filepath.Join(dir, "entries", filename))

	// Get should return cache miss and clean up metadata
	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("expected error for missing entry")
	}

	size, _ := cache.Size(ctx)
	if size != 0 {
		t.Errorf("missing entry should have been removed from metadata, got size %d", size)
	}
}

func TestKeyToFilename(t *testing.T) {
	// Same key should produce same filename
	f1 := keyToFilename("test-key")
	f2 := keyToFilename("test-key")
	if f1 != f2 {
		t.Errorf("same key should produce same filename: %s vs %s", f1, f2)
	}

	// Different keys should produce different filenames
	f3 := keyToFilename("different-key")
	if f1 == f3 {
		t.Error("different keys should produce different filenames")
	}

	// Filename should end with .json
	if len(f1) < 5 || f1[len(f1)-5:] != ".json" {
		t.Errorf("filename should end with .json: %s", f1)
	}
}
