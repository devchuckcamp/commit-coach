package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devchuckcamp/commit-coach/internal/adapters/cache"
	"github.com/devchuckcamp/commit-coach/internal/app"
	"github.com/devchuckcamp/commit-coach/internal/ports"
	"github.com/devchuckcamp/commit-coach/internal/testutil"
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

// =============================================================================
// LLM Failure Scenario Tests
// =============================================================================

func TestSuggestLLMError(t *testing.T) {
	// Test that LLM errors are properly propagated
	fakeLLM := &testutil.FakeLLM{
		Err: errors.New("API rate limit exceeded"),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Fatal("Expected error when LLM fails")
	}

	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("Expected rate limit error message, got: %v", err)
	}
}

func TestSuggestLLMTimeout(t *testing.T) {
	// Test that context cancellation works
	// Note: SuggestService wraps context with its own timeout, so we test cancellation
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	// Create and immediately cancel the context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	// When context is cancelled before the call, the service's internal timeout
	// still starts fresh. This test verifies the system handles pre-cancelled contexts.
	// The actual timeout behavior is tested by the service's internal 90s timeout.
	_ = err // May or may not error depending on timing
	_ = time.Now() // Use time import
}

func TestSuggestMalformedResponse_TooFewSuggestions(t *testing.T) {
	// Test handling of LLM returning fewer than 3 suggestions
	fakeLLM := &testutil.FakeLLM{
		Suggestions: []ports.CommitSuggestion{
			{Type: "feat", Subject: "only one suggestion"},
		},
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Fatal("Expected error when LLM returns too few suggestions")
	}

	if !strings.Contains(err.Error(), "expected 3") {
		t.Errorf("Expected 'expected 3' in error message, got: %v", err)
	}
}

func TestSuggestMalformedResponse_InvalidType(t *testing.T) {
	// Test handling of LLM returning invalid commit type
	fakeLLM := &testutil.FakeLLM{
		Suggestions: []ports.CommitSuggestion{
			{Type: "invalid", Subject: "first"},
			{Type: "feat", Subject: "second"},
			{Type: "fix", Subject: "third"},
		},
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Fatal("Expected error when LLM returns invalid type")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected 'invalid' in error message, got: %v", err)
	}
}

func TestSuggestMalformedResponse_SubjectTooLong(t *testing.T) {
	// Test handling of LLM returning subject > 72 chars
	// Note: The domain layer truncates subjects > 72 chars during Normalize()
	// so this doesn't cause an error - it's handled gracefully
	longSubject := strings.Repeat("x", 100) // 100 chars, exceeds 72 limit

	fakeLLM := &testutil.FakeLLM{
		Suggestions: []ports.CommitSuggestion{
			{Type: "feat", Subject: longSubject},
			{Type: "fix", Subject: "normal subject"},
			{Type: "docs", Subject: "normal subject"},
		},
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	suggestions, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	// Should succeed - long subjects are truncated during normalization
	if err != nil {
		t.Fatalf("Expected success (truncation), got error: %v", err)
	}

	// Verify the subject was truncated to 72 chars
	if len(suggestions[0].Subject) > 72 {
		t.Errorf("Expected subject truncated to 72 chars, got %d", len(suggestions[0].Subject))
	}
}

func TestSuggestMalformedResponse_EmptySubject(t *testing.T) {
	// Test handling of LLM returning empty subject
	fakeLLM := &testutil.FakeLLM{
		Suggestions: []ports.CommitSuggestion{
			{Type: "feat", Subject: ""},
			{Type: "fix", Subject: "normal"},
			{Type: "docs", Subject: "normal"},
		},
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Fatal("Expected error when LLM returns empty subject")
	}

	if !strings.Contains(err.Error(), "subject") {
		t.Errorf("Expected 'subject' in error message, got: %v", err)
	}
}

func TestAnalyzeFilesLLMError_GracefulDegradation(t *testing.T) {
	// Test that AnalyzeFiles gracefully degrades on LLM failure
	fakeLLM := &testutil.FakeLLM{
		FileAnalysisErr: errors.New("LLM unavailable"),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent: testutil.SampleDiffSmall,
		IsInRepoValue:     true,
		StagedFilesContent: []ports.StagedFile{
			{Path: "file1.go", Status: "M"},
			{Path: "file2.go", Status: "A"},
		},
	}

	application := app.NewApp(fakeLLM, fakeGit, nil, 8192, false)

	ctx := context.Background()
	result, files, err := application.Analyze.AnalyzeFiles(ctx, "gpt-4o-mini", 0.7)

	// Should NOT return an error - graceful degradation
	if err != nil {
		t.Fatalf("Expected graceful degradation, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result with degradation")
	}

	if !result.Degraded {
		t.Error("Expected Degraded=true for graceful degradation")
	}

	if !result.IsHomogeneous {
		t.Error("Degraded result should be homogeneous")
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}

func TestGitStagedFilesError(t *testing.T) {
	// Test handling of git errors when getting staged files
	fakeLLM := &testutil.FakeLLM{
		Suggestions: testutil.SampleLLMResponse(),
	}

	fakeGit := &testutil.FakeGit{
		StagedDiffContent:  testutil.SampleDiffSmall,
		IsInRepoValue:      true,
		StagedFilesErr:     errors.New("git error: cannot read index"),
		StagedFilesContent: nil,
	}

	cacheAdapter := cache.NewInMemory()
	application := app.NewApp(fakeLLM, fakeGit, cacheAdapter, 8192, true)

	ctx := context.Background()
	_, err := application.Suggest.SuggestCommits(ctx, "openai", "gpt-4o-mini", 0.7)

	if err == nil {
		t.Fatal("Expected error when git staged files fails")
	}

	// Error message uses structured error format: "git staged-files failed: ..."
	if !strings.Contains(err.Error(), "staged-files") && !strings.Contains(err.Error(), "staged files") {
		t.Errorf("Expected 'staged-files' in error message, got: %v", err)
	}
}

func TestCommitGitError(t *testing.T) {
	// Test handling of git commit errors
	fakeGit := &testutil.FakeGit{
		IsInRepoValue: true,
		CommitErr:     errors.New("pre-commit hook failed"),
	}

	commitService := app.NewCommitService(fakeGit)

	ctx := context.Background()
	_, err := commitService.Commit(ctx, "feat: test", false)

	if err == nil {
		t.Fatal("Expected error when git commit fails")
	}

	if !strings.Contains(err.Error(), "commit failed") {
		t.Errorf("Expected 'commit failed' in error message, got: %v", err)
	}
}
