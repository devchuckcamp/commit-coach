package ports

import (
	"context"
	"time"
)

// LLM is the interface for language model providers.
type LLM interface {
	SuggestCommits(ctx context.Context, input SuggestInput) ([]CommitSuggestion, error)
}

// SuggestInput is the input to LLM.SuggestCommits.
type SuggestInput struct {
	StagedDiff string
	FileList   []string
	Model      string
	Temperature float32
	Options    map[string]interface{} // provider-specific options
}

// CommitSuggestion is a single commit suggestion from the LLM.
type CommitSuggestion struct {
	Type    string // "feat", "fix", "docs", etc.
	Subject string // max 72 chars
	Body    string // optional, multiline
	Footer  string // optional, "BREAKING CHANGE: ..."
}

// Git is the interface for git operations.
type Git interface {
	StagedDiff(ctx context.Context) (string, error)
	Commit(ctx context.Context, message string, dryRun bool) (hash string, err error)
	IsInRepository(ctx context.Context) (bool, error)
}

// Redactor redacts sensitive data from text.
type Redactor interface {
	Redact(text string) string
	RedactLog(text string) string // for logging (more aggressive)
}

// Clock provides current time (mockable).
type Clock interface {
	Now() time.Time
}

// Cache caches suggestions by diff hash.
type Cache interface {
	Get(ctx context.Context, key string) ([]CommitSuggestion, error)
	Set(ctx context.Context, key string, suggestions []CommitSuggestion) error
}
