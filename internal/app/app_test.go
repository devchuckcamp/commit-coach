package app

import (
	"context"
	"errors"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
	"github.com/devchuckcamp/commit-coach/internal/testutil"
)

func TestSuggestService_SuggestCommits(t *testing.T) {
	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "add new feature"},
		{Type: "fix", Subject: "fix a bug"},
		{Type: "docs", Subject: "update documentation"},
	}

	llm := &testutil.FakeLLM{Suggestions: suggestions}
	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "diff --git a/main.go\n+new line",
		StagedFilesContent: []ports.StagedFile{
			{Path: "main.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}
	cache := testutil.NewFakeCache()

	svc := NewSuggestService(llm, git, redactor, cache, 10000, true)

	result, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(result))
	}

	if llm.CallCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.CallCount)
	}
}

func TestSuggestService_NotInRepository(t *testing.T) {
	llm := &testutil.FakeLLM{}
	git := &testutil.FakeGit{IsInRepoValue: false}
	redactor := &testutil.FakeRedactor{}

	svc := NewSuggestService(llm, git, redactor, nil, 10000, false)

	_, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err == nil {
		t.Fatal("expected error when not in repository")
	}

	if llm.CallCount != 0 {
		t.Error("LLM should not be called when not in repository")
	}
}

func TestSuggestService_NoStagedChanges(t *testing.T) {
	llm := &testutil.FakeLLM{}
	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "",
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewSuggestService(llm, git, redactor, nil, 10000, false)

	_, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err == nil {
		t.Fatal("expected error when no staged changes")
	}

	if llm.CallCount != 0 {
		t.Error("LLM should not be called when no staged changes")
	}
}

func TestSuggestService_CacheHit(t *testing.T) {
	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "cached suggestion one"},
		{Type: "fix", Subject: "cached suggestion two"},
		{Type: "docs", Subject: "cached suggestion three"},
	}

	llm := &testutil.FakeLLM{Suggestions: suggestions}
	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "diff content",
		StagedFilesContent: []ports.StagedFile{
			{Path: "file.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}
	cache := testutil.NewFakeCache()

	svc := NewSuggestService(llm, git, redactor, cache, 10000, true)

	// First call - cache miss
	result1, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call - should hit cache
	result2, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if llm.CallCount != 1 {
		t.Errorf("expected 1 LLM call (cache hit), got %d", llm.CallCount)
	}

	if len(result1) != len(result2) {
		t.Error("cached result should match original")
	}
}

func TestSuggestService_LLMError(t *testing.T) {
	llm := &testutil.FakeLLM{Err: errors.New("API error")}
	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "diff content",
		StagedFilesContent: []ports.StagedFile{
			{Path: "file.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewSuggestService(llm, git, redactor, nil, 10000, false)

	_, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err == nil {
		t.Fatal("expected error on LLM failure")
	}
}

func TestSuggestService_DiffCapping(t *testing.T) {
	suggestions := []ports.CommitSuggestion{
		{Type: "feat", Subject: "a"},
		{Type: "fix", Subject: "b"},
		{Type: "docs", Subject: "c"},
	}

	llm := &testutil.FakeLLM{Suggestions: suggestions}
	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "this is a very long diff that exceeds the cap",
		StagedFilesContent: []ports.StagedFile{
			{Path: "file.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}

	// Set a very small cap
	svc := NewSuggestService(llm, git, redactor, nil, 10, false)

	_, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	// LLM should still be called even with capped diff
	if llm.CallCount != 1 {
		t.Error("LLM should be called with capped diff")
	}
}

func TestSuggestService_SetLLM(t *testing.T) {
	llm1 := &testutil.FakeLLM{}
	llm2 := &testutil.FakeLLM{Suggestions: []ports.CommitSuggestion{
		{Type: "feat", Subject: "from llm2"},
		{Type: "fix", Subject: "from llm2"},
		{Type: "docs", Subject: "from llm2"},
	}}

	git := &testutil.FakeGit{
		IsInRepoValue:     true,
		StagedDiffContent: "diff",
		StagedFilesContent: []ports.StagedFile{
			{Path: "file.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewSuggestService(llm1, git, redactor, nil, 10000, false)

	// Swap LLM
	svc.SetLLM(llm2)

	// Should not panic with nil
	svc.SetLLM(nil)

	result, err := svc.SuggestCommits(context.Background(), "openai", "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	// llm2 should be used
	if llm2.CallCount != 1 {
		t.Errorf("expected llm2 to be called, got %d calls", llm2.CallCount)
	}

	if result[0].Subject != "from llm2" {
		t.Error("result should come from llm2")
	}
}

func TestCommitService_Commit(t *testing.T) {
	git := &testutil.FakeGit{}
	svc := NewCommitService(git)

	hash, err := svc.Commit(context.Background(), "feat: test commit", false)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if len(git.CommittedMessages) != 1 {
		t.Errorf("expected 1 committed message, got %d", len(git.CommittedMessages))
	}

	if git.CommittedMessages[0] != "feat: test commit" {
		t.Errorf("committed message mismatch: %q", git.CommittedMessages[0])
	}
}

func TestCommitService_DryRun(t *testing.T) {
	git := &testutil.FakeGit{}
	svc := NewCommitService(git)

	_, err := svc.Commit(context.Background(), "feat: dry run", true)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(git.CommittedMessages) != 0 {
		t.Error("dry run should not commit")
	}
}

func TestCommitService_EmptyMessage(t *testing.T) {
	git := &testutil.FakeGit{}
	svc := NewCommitService(git)

	_, err := svc.Commit(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error on empty message")
	}
}

func TestCommitService_GitError(t *testing.T) {
	git := &testutil.FakeGit{CommitErr: errors.New("git error")}
	svc := NewCommitService(git)

	_, err := svc.Commit(context.Background(), "feat: test", false)
	if err == nil {
		t.Fatal("expected error on git failure")
	}
}

func TestAnalyzeService_AnalyzeFiles(t *testing.T) {
	llm := &testutil.FakeLLM{
		FileAnalysisResult: &ports.FileAnalysisResult{
			IsHomogeneous: true,
			Groups: []ports.FileGroup{
				{Label: "All files", Files: []string{"a.go", "b.go"}, Confidence: 0.9},
			},
			Reasoning: "Files are related",
		},
	}
	git := &testutil.FakeGit{
		StagedFilesContent: []ports.StagedFile{
			{Path: "a.go", Status: "M"},
			{Path: "b.go", Status: "A"},
		},
		StagedDiffContent: "diff content",
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewAnalyzeService(llm, git, redactor, 10000)

	result, files, err := svc.AnalyzeFiles(context.Background(), "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("AnalyzeFiles failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	if !result.IsHomogeneous {
		t.Error("expected homogeneous result")
	}
}

func TestAnalyzeService_SkipsSingleFile(t *testing.T) {
	llm := &testutil.FakeLLM{}
	git := &testutil.FakeGit{
		StagedFilesContent: []ports.StagedFile{
			{Path: "single.go", Status: "M"},
		},
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewAnalyzeService(llm, git, redactor, 10000)

	result, files, err := svc.AnalyzeFiles(context.Background(), "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("AnalyzeFiles failed: %v", err)
	}

	// Should skip analysis for single file
	if result != nil {
		t.Error("expected nil result for single file")
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestAnalyzeService_GracefulDegradation(t *testing.T) {
	llm := &testutil.FakeLLM{
		FileAnalysisErr: errors.New("LLM error"),
	}
	git := &testutil.FakeGit{
		StagedFilesContent: []ports.StagedFile{
			{Path: "a.go", Status: "M"},
			{Path: "b.go", Status: "A"},
		},
		StagedDiffContent: "diff content",
	}
	redactor := &testutil.FakeRedactor{}

	svc := NewAnalyzeService(llm, git, redactor, 10000)

	result, files, err := svc.AnalyzeFiles(context.Background(), "gpt-4", 0.7)
	if err != nil {
		t.Fatalf("AnalyzeFiles should not fail on LLM error: %v", err)
	}

	// Should return graceful degradation result
	if result == nil {
		t.Fatal("expected non-nil fallback result")
	}

	if !result.IsHomogeneous {
		t.Error("fallback should be homogeneous")
	}

	if !result.Degraded {
		t.Error("fallback result should have Degraded=true")
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestUnstageService_Unstage(t *testing.T) {
	git := &testutil.FakeGit{}
	svc := NewUnstageService(git)

	err := svc.Unstage(context.Background(), []string{"file1.go", "file2.go"})
	if err != nil {
		t.Fatalf("Unstage failed: %v", err)
	}

	if len(git.UnstagedFiles) != 2 {
		t.Errorf("expected 2 unstaged files, got %d", len(git.UnstagedFiles))
	}
}

func TestUnstageService_EmptyList(t *testing.T) {
	git := &testutil.FakeGit{}
	svc := NewUnstageService(git)

	err := svc.Unstage(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Unstage with empty list should not fail: %v", err)
	}

	if len(git.UnstagedFiles) != 0 {
		t.Error("should not unstage anything for empty list")
	}
}

func TestNewApp(t *testing.T) {
	llm := &testutil.FakeLLM{}
	git := &testutil.FakeGit{}
	cache := testutil.NewFakeCache()

	app := NewApp(llm, git, cache, 10000, true)

	if app.Suggest == nil {
		t.Error("Suggest service should not be nil")
	}
	if app.Commit == nil {
		t.Error("Commit service should not be nil")
	}
	if app.Analyze == nil {
		t.Error("Analyze service should not be nil")
	}
	if app.Unstage == nil {
		t.Error("Unstage service should not be nil")
	}
	if app.Redactor == nil {
		t.Error("Redactor should not be nil")
	}
}

func TestSuggestService_CapDiff(t *testing.T) {
	svc := &SuggestService{}

	tests := []struct {
		name     string
		diff     string
		maxBytes int
		expected string
	}{
		{
			name:     "under limit unchanged",
			diff:     "line1\nline2\n",
			maxBytes: 100,
			expected: "line1\nline2\n",
		},
		{
			name:     "truncates at line boundary",
			diff:     "line1\nline2\nline3\n",
			maxBytes: 12, // "line1\nline2\n" is 12 bytes
			expected: "line1\nline2\n",
		},
		{
			name:     "finds last newline before limit",
			diff:     "line1\nline2\nline3\n",
			maxBytes: 15, // mid-way through "line3"
			expected: "line1\nline2\n",
		},
		{
			name:     "single long line falls back to byte truncation",
			diff:     "verylonglinewithnonewlines",
			maxBytes: 10,
			expected: "verylongli",
		},
		{
			name:     "empty diff",
			diff:     "",
			maxBytes: 10,
			expected: "",
		},
		{
			name:     "exact size",
			diff:     "exact\n",
			maxBytes: 6,
			expected: "exact\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.capDiff(tt.diff, tt.maxBytes)
			if got != tt.expected {
				t.Errorf("capDiff(%q, %d) = %q, want %q", tt.diff, tt.maxBytes, got, tt.expected)
			}
		})
	}
}
