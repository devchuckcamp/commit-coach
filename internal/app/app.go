package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/devchuckcamp/commit-coach/internal/domain"
	"github.com/devchuckcamp/commit-coach/internal/ports"
	"github.com/devchuckcamp/commit-coach/internal/security"
)

// SuggestService generates commit suggestions.
type SuggestService struct {
	llm       ports.LLM
	git       ports.Git
	redactor  ports.Redactor
	cache     ports.Cache
	diffCap   int
	timeout   time.Duration
	useCache  bool
}

// NewSuggestService creates a new suggestion service.
func NewSuggestService(llm ports.LLM, git ports.Git, redactor ports.Redactor, cache ports.Cache, diffCap int, useCache bool) *SuggestService {
	return &SuggestService{
		llm:      llm,
		git:      git,
		redactor: redactor,
		cache:    cache,
		diffCap:  diffCap,
		timeout:  90 * time.Second,
		useCache: useCache,
	}
}

// SuggestCommits generates 3 commit suggestions based on staged diff.
func (s *SuggestService) SuggestCommits(ctx context.Context, provider, model string, temperature float32) ([]domain.Suggestion, error) {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Step 1: Check if in repository
	inRepo, err := s.git.IsInRepository(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check repository status: %w", err)
	}
	if !inRepo {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Step 2: Get staged diff
	diff, err := s.git.StagedDiff(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read staged diff: %w", err)
	}
	if diff == "" {
		return nil, fmt.Errorf("no staged changes")
	}

	// Step 3: Check cache
	diffHash := s.hashDiff(diff, provider, model)
	if s.useCache && s.cache != nil {
		if cached, err := s.cache.Get(ctx, diffHash); err == nil {
			return s.validateAndNormalize(cached)
		}
	}

	// Step 4: Cap and redact diff
	cappedDiff := s.capDiff(diff, s.diffCap)
	redactedDiff := s.redactor.Redact(cappedDiff)

	// Step 5: Build file list
	fileList := []string{} // TODO: extract from diff

	// Step 6: Call LLM
	input := ports.SuggestInput{
		StagedDiff:  redactedDiff,
		FileList:    fileList,
		Model:       model,
		Temperature: temperature,
	}

	llmSuggestions, err := s.llm.SuggestCommits(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 7: Validate suggestions
	result, err := s.validateAndNormalize(llmSuggestions)
	if err != nil {
		return nil, fmt.Errorf("invalid suggestions from LLM: %w", err)
	}

	// Step 8: Cache result
	if s.useCache && s.cache != nil {
		_ = s.cache.Set(ctx, diffHash, llmSuggestions) // ignore cache errors
	}

	return result, nil
}

// SetLLM swaps the LLM implementation used by this service.
// Safe to call from the Bubble Tea Update loop (single-owner).
func (s *SuggestService) SetLLM(llm ports.LLM) {
	if llm == nil {
		return
	}
	s.llm = llm
}

// hashDiff computes a SHA256 hash of the diff plus a cache namespace.
func (s *SuggestService) hashDiff(diff, provider, model string) string {
	h := sha256.New()
	io.WriteString(h, diff)
	io.WriteString(h, "\nprovider=")
	io.WriteString(h, provider)
	io.WriteString(h, "\nmodel=")
	io.WriteString(h, model)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// capDiff truncates diff to max size.
func (s *SuggestService) capDiff(diff string, maxBytes int) string {
	if len(diff) <= maxBytes {
		return diff
	}
	return diff[:maxBytes]
}

// validateAndNormalize converts port suggestions to domain suggestions with validation.
func (s *SuggestService) validateAndNormalize(portSuggestions []ports.CommitSuggestion) ([]domain.Suggestion, error) {
	if len(portSuggestions) < 3 {
		return nil, fmt.Errorf("expected 3 suggestions, got %d", len(portSuggestions))
	}

	result := make([]domain.Suggestion, 3)
	for i := 0; i < 3; i++ {
		ps := portSuggestions[i]
		ds := domain.Suggestion{
			Type:    ps.Type,
			Subject: ps.Subject,
			Body:    ps.Body,
			Footer:  ps.Footer,
		}
		ds.Normalize()
		if err := ds.Validate(); err != nil {
			return nil, fmt.Errorf("suggestion %d validation failed: %w", i, err)
		}
		result[i] = ds
	}

	return result, nil
}

// CommitService handles commit execution.
type CommitService struct {
	git     ports.Git
	timeout time.Duration
}

// NewCommitService creates a new commit service.
func NewCommitService(git ports.Git) *CommitService {
	return &CommitService{
		git:     git,
		timeout: 10 * time.Second,
	}
}

// Commit executes a git commit with the given message (atomically).
func (c *CommitService) Commit(ctx context.Context, message string, dryRun bool) (hash string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Validate message before attempting commit
	if message == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	// Attempt commit
	hash, err = c.git.Commit(ctx, message, dryRun)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	return hash, nil
}

// AnalyzeService handles file relation analysis.
type AnalyzeService struct {
	llm      ports.LLM
	git      ports.Git
	redactor ports.Redactor
	diffCap  int
	timeout  time.Duration
}

// NewAnalyzeService creates a new analyze service.
func NewAnalyzeService(llm ports.LLM, git ports.Git, redactor ports.Redactor, diffCap int) *AnalyzeService {
	return &AnalyzeService{
		llm:      llm,
		git:      git,
		redactor: redactor,
		diffCap:  diffCap,
		timeout:  60 * time.Second,
	}
}

// AnalyzeFiles analyzes if staged files belong in the same commit.
// Returns nil (not an error) if analysis should be skipped (e.g., <= 1 file).
// On LLM failure, returns a homogeneous result (graceful degradation).
func (a *AnalyzeService) AnalyzeFiles(ctx context.Context, model string, temperature float32) (*ports.FileAnalysisResult, []ports.StagedFile, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Get staged files
	files, err := a.git.StagedFiles(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get staged files: %w", err)
	}

	// Skip analysis if <= 1 file
	if len(files) <= 1 {
		return nil, files, nil
	}

	// Get diff for context
	diff, err := a.git.StagedDiff(ctx)
	if err != nil {
		return nil, files, fmt.Errorf("failed to get staged diff: %w", err)
	}

	// Cap and redact diff
	cappedDiff := a.capDiff(diff, a.diffCap)
	redactedDiff := a.redactor.Redact(cappedDiff)

	// Build input
	input := ports.FileAnalysisInput{
		Files:       files,
		DiffContent: redactedDiff,
		Model:       model,
		Temperature: temperature,
	}

	// Call LLM for analysis
	result, err := a.llm.AnalyzeFileRelations(ctx, input)
	if err != nil {
		// Graceful degradation: on LLM failure, assume files are related
		allFiles := make([]string, len(files))
		for i, f := range files {
			allFiles[i] = f.Path
		}
		return &ports.FileAnalysisResult{
			IsHomogeneous: true,
			Groups: []ports.FileGroup{
				{
					Label:      "All staged files",
					Files:      allFiles,
					Confidence: 0.5,
				},
			},
			Reasoning: "Analysis unavailable; defaulting to homogeneous.",
		}, files, nil
	}

	return result, files, nil
}

// SetLLM swaps the LLM implementation.
func (a *AnalyzeService) SetLLM(llm ports.LLM) {
	if llm == nil {
		return
	}
	a.llm = llm
}

func (a *AnalyzeService) capDiff(diff string, maxBytes int) string {
	if len(diff) <= maxBytes {
		return diff
	}
	return diff[:maxBytes]
}

// UnstageService handles file unstaging.
type UnstageService struct {
	git     ports.Git
	timeout time.Duration
}

// NewUnstageService creates a new unstage service.
func NewUnstageService(git ports.Git) *UnstageService {
	return &UnstageService{
		git:     git,
		timeout: 10 * time.Second,
	}
}

// Unstage removes files from the staging area.
func (u *UnstageService) Unstage(ctx context.Context, files []string) error {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	if len(files) == 0 {
		return nil
	}

	return u.git.Unstage(ctx, files)
}

// App is the application container with all services.
type App struct {
	Suggest  *SuggestService
	Commit   *CommitService
	Analyze  *AnalyzeService
	Unstage  *UnstageService
	Redactor ports.Redactor
}

// NewApp creates a new application with all dependencies wired.
func NewApp(llm ports.LLM, git ports.Git, cache ports.Cache, diffCap int, useCache bool) *App {
	redactor := security.NewRedactor()
	return &App{
		Suggest:  NewSuggestService(llm, git, redactor, cache, diffCap, useCache),
		Commit:   NewCommitService(git),
		Analyze:  NewAnalyzeService(llm, git, redactor, diffCap),
		Unstage:  NewUnstageService(git),
		Redactor: redactor,
	}
}
