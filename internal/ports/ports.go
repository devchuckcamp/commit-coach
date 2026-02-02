package ports

import (
	"context"
	"time"
)

// LLM is the interface for language model providers.
type LLM interface {
	SuggestCommits(ctx context.Context, input SuggestInput) ([]CommitSuggestion, error)
	AnalyzeFileRelations(ctx context.Context, input FileAnalysisInput) (*FileAnalysisResult, error)
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
	StagedFiles(ctx context.Context) ([]StagedFile, error)
	Unstage(ctx context.Context, files []string) error
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

// StagedFile represents a staged file with metadata.
type StagedFile struct {
	Path   string // file path
	Status string // "A" (added), "M" (modified), "D" (deleted), "R" (renamed)
}

// FileGroup represents a logical grouping of related files.
type FileGroup struct {
	Label      string   // e.g., "Authentication changes"
	Files      []string // file paths in this group
	Confidence float32  // 0.0-1.0
}

// FileAnalysisResult contains the analysis output.
type FileAnalysisResult struct {
	IsHomogeneous bool        // true if all files are related
	Groups        []FileGroup // logical groupings
	Reasoning     string      // explanation
}

// FileAnalysisInput is the input for file relation analysis.
type FileAnalysisInput struct {
	Files       []StagedFile
	DiffContent string // redacted diff for context
	Model       string
	Temperature float32
}
