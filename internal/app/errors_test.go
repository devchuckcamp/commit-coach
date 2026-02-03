package app

import (
	"errors"
	"testing"
)

func TestLLMError(t *testing.T) {
	originalErr := errors.New("API rate limit")
	llmErr := NewLLMError("openai", "gpt-4", originalErr)

	// Test Error() method
	errStr := llmErr.Error()
	if errStr == "" {
		t.Error("Error() returned empty string")
	}
	if !contains(errStr, "openai") {
		t.Error("Error() should contain provider name")
	}
	if !contains(errStr, "gpt-4") {
		t.Error("Error() should contain model name")
	}
	if !contains(errStr, "rate limit") {
		t.Error("Error() should contain original error message")
	}

	// Test Unwrap()
	if llmErr.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}

	// Test IsLLMError()
	if !IsLLMError(llmErr) {
		t.Error("IsLLMError should return true for LLMError")
	}
	if IsLLMError(originalErr) {
		t.Error("IsLLMError should return false for regular error")
	}
}

func TestValidationError(t *testing.T) {
	valErr := NewValidationError(1, "type", "invalid commit type")

	errStr := valErr.Error()
	if !contains(errStr, "1") {
		t.Error("Error() should contain index")
	}
	if !contains(errStr, "type") {
		t.Error("Error() should contain field")
	}
	if !contains(errStr, "invalid") {
		t.Error("Error() should contain message")
	}

	if !IsValidationError(valErr) {
		t.Error("IsValidationError should return true for ValidationError")
	}
	if IsValidationError(errors.New("other")) {
		t.Error("IsValidationError should return false for regular error")
	}
}

func TestGitError(t *testing.T) {
	originalErr := errors.New("exit status 1")
	gitErr := NewGitError("commit", originalErr)

	errStr := gitErr.Error()
	if !contains(errStr, "commit") {
		t.Error("Error() should contain operation")
	}
	if !contains(errStr, "exit status") {
		t.Error("Error() should contain original error message")
	}

	if gitErr.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}

	if !IsGitError(gitErr) {
		t.Error("IsGitError should return true for GitError")
	}
	if IsGitError(originalErr) {
		t.Error("IsGitError should return false for regular error")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Test ErrNotInRepository
	if !IsNotInRepository(ErrNotInRepository) {
		t.Error("IsNotInRepository should return true for ErrNotInRepository")
	}
	if IsNotInRepository(errors.New("other")) {
		t.Error("IsNotInRepository should return false for other errors")
	}

	// Test ErrNoStagedChanges
	if !IsNoStagedChanges(ErrNoStagedChanges) {
		t.Error("IsNoStagedChanges should return true for ErrNoStagedChanges")
	}
	if IsNoStagedChanges(errors.New("other")) {
		t.Error("IsNoStagedChanges should return false for other errors")
	}

	// Test wrapped errors
	wrapped := errors.Join(errors.New("context"), ErrNotInRepository)
	if !IsNotInRepository(wrapped) {
		t.Error("IsNotInRepository should work with wrapped errors")
	}
}

func TestErrorMessages(t *testing.T) {
	// Ensure error messages are human-readable
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotInRepository", ErrNotInRepository},
		{"ErrNoStagedChanges", ErrNoStagedChanges},
		{"ErrEmptyMessage", ErrEmptyMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() == "" {
				t.Errorf("%s has empty message", tt.name)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
