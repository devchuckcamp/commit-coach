package integration

import (
	"context"
	"testing"

	"github.com/chuckie/commit-coach/internal/adapters/cache"
	"github.com/chuckie/commit-coach/internal/app"
	"github.com/chuckie/commit-coach/internal/ports"
	"github.com/chuckie/commit-coach/internal/testutil"
)

func TestSuggestWorkflow(t *testing.T) {
	// Setup: fake LLM + git + cache
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
	}

	cacheAdapter := cache.NewInMemory()

	app := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	// Action: generate suggestions
	ctx := context.Background()
	suggestions, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	// Assert
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	if len(suggestions) != 3 {
		t.Errorf("Expected 3 suggestions, got %d", len(suggestions))
	}

	for i, s := range suggestions {
		if err := s.Validate(); err != nil {
			t.Errorf("Suggestion %d invalid: %v", i, err)
		}
	}
}

func TestSuggestWithCache(t *testing.T) {
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
		CallCount:   0,
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
	}

	cacheAdapter := cache.NewInMemory()
	app := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()

	// First call: should hit LLM
	_, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)
	if err != nil {
		t.Fatalf("First SuggestCommits failed: %v", err)
	}

	firstCallCount := fakeLLM.CallCount
	if firstCallCount != 1 {
		t.Errorf("Expected 1 LLM call, got %d", firstCallCount)
	}

	// Second call: should hit cache
	_, err = app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)
	if err != nil {
		t.Fatalf("Second SuggestCommits failed: %v", err)
	}

	if fakeLLM.CallCount != 1 {
		t.Errorf("Expected 1 LLM call (cache hit), got %d", fakeLLM.CallCount)
	}

	if cacheAdapter.Size() != 1 {
		t.Errorf("Expected 1 cached entry, got %d", cacheAdapter.Size())
	}
}

func TestSuggestNoStagedChanges(t *testing.T) {
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: "", // Empty diff
		IsInRepoValue:     true,
	}

	cacheAdapter := cache.NewInMemory()
	app := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Error("Expected error for empty staged diff")
	}
}

func TestSuggestNotInRepo(t *testing.T) {
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     false, // Not in repo
	}

	cacheAdapter := cache.NewInMemory()
	app := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Error("Expected error when not in repository")
	}
}

func TestCommitWorkflow(t *testing.T) {
	fakeGit := &testutil.FakeGit{
		IsInRepoValue: true,
	}

	commitService := app.NewCommitService(fakeGit)

	ctx := context.Background()
	message := "feat: add new feature"

	hash, err := commitService.Commit(ctx, message, false)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if hash == "" {
		t.Error("Expected non-empty commit hash")
	}

	if len(fakeGit.CommittedMessages) != 1 {
		t.Errorf("Expected 1 committed message, got %d", len(fakeGit.CommittedMessages))
	}

	if fakeGit.CommittedMessages[0] != message {
		t.Errorf("Committed message mismatch: got %s, want %s", fakeGit.CommittedMessages[0], message)
	}
}

func TestCommitDryRun(t *testing.T) {
	fakeGit := &testutil.FakeGit{
		IsInRepoValue: true,
	}

	commitService := app.NewCommitService(fakeGit)

	ctx := context.Background()
	message := "feat: add new feature"

	_, err := commitService.Commit(ctx, message, true)
	if err != nil {
		t.Fatalf("Dry-run commit failed: %v", err)
	}

	if len(fakeGit.CommittedMessages) != 0 {
		t.Error("Expected no commits in dry-run mode")
	}
}

func TestCommitEmptyMessage(t *testing.T) {
	fakeGit := &testutil.FakeGit{
		IsInRepoValue: true,
	}

	commitService := app.NewCommitService(fakeGit)

	ctx := context.Background()
	_, err := commitService.Commit(ctx, "", false)

	if err == nil {
		t.Error("Expected error for empty commit message")
	}
}

func TestSuggestionValidationInOrchestrator(t *testing.T) {
	// Test that invalid LLM responses are rejected
	fakeLLM := &testutil.FakeLLM{
		Suggestions: []ports.CommitSuggestion{
			{
				Type:    "invalid_type",
				Subject: "bad suggestion",
				Body:    "",
				Footer:  "",
			},
		},
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
	}

	cacheAdapter := cache.NewInMemory()
	app := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Error("Expected error for invalid suggestion type")
	}
}

func TestDiffCapcing(t *testing.T) {
	// Test that large diffs are capped
	largeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffLarge,
		IsInRepoValue:     true,
	}

	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	cacheAdapter := cache.NewInMemory()
	app := app.NewApp(fakeLLM, largeGit, cacheAdapter, 100, true) // Small cap

	ctx := context.Background()
	_, err := app.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	// Should still work, but diff will be capped
	if err != nil {
		t.Fatalf("Expected success with capped diff, got error: %v", err)
	}
}
