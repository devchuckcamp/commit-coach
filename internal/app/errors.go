package app

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure conditions.
var (
	// ErrNotInRepository indicates the command was run outside a git repository.
	ErrNotInRepository = errors.New("not in a git repository")

	// ErrNoStagedChanges indicates there are no staged changes to commit.
	ErrNoStagedChanges = errors.New("no staged changes")

	// ErrEmptyMessage indicates an empty commit message was provided.
	ErrEmptyMessage = errors.New("commit message cannot be empty")
)

// LLMError represents an error from the LLM provider.
type LLMError struct {
	Provider string
	Model    string
	Err      error
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("LLM error (provider=%s, model=%s): %v", e.Provider, e.Model, e.Err)
}

func (e *LLMError) Unwrap() error {
	return e.Err
}

// NewLLMError creates a new LLM error with provider context.
func NewLLMError(provider, model string, err error) *LLMError {
	return &LLMError{
		Provider: provider,
		Model:    model,
		Err:      err,
	}
}

// ValidationError represents a validation failure for LLM suggestions.
type ValidationError struct {
	Index   int    // which suggestion failed (0-indexed)
	Field   string // which field failed validation
	Message string // description of the failure
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("suggestion %d validation failed: %s: %s", e.Index, e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(index int, field, message string) *ValidationError {
	return &ValidationError{
		Index:   index,
		Field:   field,
		Message: message,
	}
}

// GitError represents an error from git operations.
type GitError struct {
	Operation string // e.g., "commit", "diff", "staged-files"
	Err       error
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s failed: %v", e.Operation, e.Err)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

// NewGitError creates a new git error with operation context.
func NewGitError(operation string, err error) *GitError {
	return &GitError{
		Operation: operation,
		Err:       err,
	}
}

// IsNotInRepository checks if the error is ErrNotInRepository.
func IsNotInRepository(err error) bool {
	return errors.Is(err, ErrNotInRepository)
}

// IsNoStagedChanges checks if the error is ErrNoStagedChanges.
func IsNoStagedChanges(err error) bool {
	return errors.Is(err, ErrNoStagedChanges)
}

// IsLLMError checks if the error is an LLM error.
func IsLLMError(err error) bool {
	var llmErr *LLMError
	return errors.As(err, &llmErr)
}

// IsValidationError checks if the error is a validation error.
func IsValidationError(err error) bool {
	var valErr *ValidationError
	return errors.As(err, &valErr)
}

// IsGitError checks if the error is a git error.
func IsGitError(err error) bool {
	var gitErr *GitError
	return errors.As(err, &gitErr)
}
