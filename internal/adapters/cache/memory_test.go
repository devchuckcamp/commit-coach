package cache

import (
	"context"
	"sync"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
)

func TestInMemory_GetSet(t *testing.T) {
	cache := NewInMemory()
	ctx := context.Background()

	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "add feature"},
		{Type: "fix", Subject: "fix bug"},
		{Type: "docs", Subject: "update docs"},
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

	for i, s := range got {
		if s.Type != suggestions[i].Type || s.Subject != suggestions[i].Subject {
			t.Errorf("suggestion %d mismatch: got %+v, want %+v", i, s, suggestions[i])
		}
	}
}

func TestInMemory_ReturnsCopy(t *testing.T) {
	cache := NewInMemory()
	ctx := context.Background()

	original := []ports.CommitSuggestion{
		{Type: "feat", Subject: "original"},
	}

	cache.Set(ctx, "key", original)

	// Modify original after setting
	original[0].Subject = "modified"

	// Get should return the original value, not the modified one
	got, _ := cache.Get(ctx, "key")
	if got[0].Subject != "original" {
		t.Errorf("cache should store a copy; got %q, want %q", got[0].Subject, "original")
	}

	// Modify retrieved value
	got[0].Subject = "also modified"

	// Get again should still return original
	got2, _ := cache.Get(ctx, "key")
	if got2[0].Subject != "original" {
		t.Errorf("cache should return a copy; got %q, want %q", got2[0].Subject, "original")
	}
}

func TestInMemory_Clear(t *testing.T) {
	cache := NewInMemory()
	ctx := context.Background()

	cache.Set(ctx, "key1", []ports.CommitSuggestion{{Type: "feat", Subject: "a"}})
	cache.Set(ctx, "key2", []ports.CommitSuggestion{{Type: "fix", Subject: "b"}})

	size, _ := cache.Size(ctx)
	if size != 2 {
		t.Errorf("expected size 2, got %d", size)
	}

	cache.Clear(ctx)

	size, _ = cache.Size(ctx)
	if size != 0 {
		t.Errorf("expected size 0 after clear, got %d", size)
	}

	_, err := cache.Get(ctx, "key1")
	if err == nil {
		t.Error("expected cache miss after clear")
	}
}

func TestInMemory_Size(t *testing.T) {
	cache := NewInMemory()
	ctx := context.Background()

	size, _ := cache.Size(ctx)
	if size != 0 {
		t.Errorf("expected size 0 for new cache, got %d", size)
	}

	cache.Set(ctx, "key1", []ports.CommitSuggestion{})
	size, _ = cache.Size(ctx)
	if size != 1 {
		t.Errorf("expected size 1, got %d", size)
	}

	cache.Set(ctx, "key2", []ports.CommitSuggestion{})
	size, _ = cache.Size(ctx)
	if size != 2 {
		t.Errorf("expected size 2, got %d", size)
	}

	// Overwrite existing key
	cache.Set(ctx, "key1", []ports.CommitSuggestion{{Type: "new", Subject: "value"}})
	size, _ = cache.Size(ctx)
	if size != 2 {
		t.Errorf("expected size 2 after overwrite, got %d", size)
	}
}

func TestInMemory_Concurrent(t *testing.T) {
	cache := NewInMemory()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			suggestions := []ports.CommitSuggestion{
				{Type: "feat", Subject: "concurrent"},
			}
			cache.Set(ctx, key, suggestions)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Get(ctx, "key")
		}()
	}

	// Concurrent size checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Size(ctx)
		}()
	}

	wg.Wait()

	// Should not panic and size should be consistent
	size, _ := cache.Size(ctx)
	if size < 0 {
		t.Error("size should not be negative")
	}
}
