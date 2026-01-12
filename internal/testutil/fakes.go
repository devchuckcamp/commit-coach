package testutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/chuckie/commit-coach/internal/ports"
)

// FakeLLM is a deterministic fake LLM for testing.
type FakeLLM struct {
	Suggestions []ports.CommitSuggestion
	Err         error
	CallCount   int
}

func (f *FakeLLM) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	f.CallCount++
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Suggestions, nil
}

// FakeGit is a fake git adapter for testing.
type FakeGit struct {
	StagedDiffContent string
	StagedDiffErr     error
	CommittedMessages []string
	CommitErr         error
	IsInRepoValue     bool
}

func (f *FakeGit) StagedDiff(ctx context.Context) (string, error) {
	if f.StagedDiffErr != nil {
		return "", f.StagedDiffErr
	}
	return f.StagedDiffContent, nil
}

func (f *FakeGit) Commit(ctx context.Context, message string, dryRun bool) (string, error) {
	if f.CommitErr != nil {
		return "", f.CommitErr
	}
	if !dryRun {
		f.CommittedMessages = append(f.CommittedMessages, message)
	}
	return "abc123def456", nil
}

func (f *FakeGit) IsInRepository(ctx context.Context) (bool, error) {
	return f.IsInRepoValue, nil
}

// FakeRedactor is a fake redactor that does nothing.
type FakeRedactor struct{}

func (f *FakeRedactor) Redact(text string) string {
	return text
}

func (f *FakeRedactor) RedactLog(text string) string {
	return text
}

// FakeCache is a simple in-memory fake cache.
type FakeCache struct {
	data map[string][]ports.CommitSuggestion
}

func NewFakeCache() *FakeCache {
	return &FakeCache{
		data: make(map[string][]ports.CommitSuggestion),
	}
}

func (f *FakeCache) Get(ctx context.Context, key string) ([]ports.CommitSuggestion, error) {
	if v, ok := f.data[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

func (f *FakeCache) Set(ctx context.Context, key string, suggestions []ports.CommitSuggestion) error {
	f.data[key] = suggestions
	return nil
}

// DiffHash computes SHA256 hash of a diff string.
func DiffHash(diff string) string {
	h := sha256.New()
	io.WriteString(h, diff)
	return fmt.Sprintf("%x", h.Sum(nil))
}
