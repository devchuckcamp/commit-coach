package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Executor implements ports.Git using os/exec.
type Executor struct {
	timeout time.Duration
}

// NewExecutor creates a new git executor.
func NewExecutor() *Executor {
	return &Executor{
		timeout: 10 * time.Second,
	}
}

// IsInRepository checks if we are in a valid git repository.
func (e *Executor) IsInRepository(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		return false, nil // Not in repo
	}
	return strings.TrimSpace(string(output)) == "true", nil
}

// StagedDiff returns the staged diff (git diff --cached --no-color).
func (e *Executor) StagedDiff(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--no-color")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(output), nil
}

// Commit runs git commit with a temp file message.
func (e *Executor) Commit(ctx context.Context, message string, dryRun bool) (string, error) {
	// Create temp file for message
	tmpFile, err := os.CreateTemp("", "commit-coach-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		// Always clean up temp file
		_ = os.Remove(tmpFile.Name())
	}()

	// Write message to temp file
	if _, err := tmpFile.WriteString(message); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write message to temp file: %w", err)
	}
	tmpFile.Close()

	// Dry run: just show what would be committed
	if dryRun {
		return "[DRY RUN] Would commit:\n" + message, nil
	}

	// Execute git commit
	cmd := exec.CommandContext(ctx, "git", "commit", "-F", tmpFile.Name())
	output, err := cmd.Output()
	if err != nil {
		// Get stderr for better error messages
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			return "", fmt.Errorf("git commit failed: %s", stderr)
		}
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	// Extract commit hash from output
	outputStr := string(output)
	hash := extractCommitHash(outputStr)
	if hash == "" {
		hash = "[commit created]" // Fallback
	}

	return hash, nil
}

// extractCommitHash attempts to extract the commit hash from git output.
// Git output typically looks like: "[branch_name hash_part] message"
func extractCommitHash(output string) string {
	// Look for pattern like "[main abc123d]"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			// Extract hash from brackets
			start := strings.Index(line, "[")
			end := strings.Index(line, "]")
			if start != -1 && end != -1 {
				content := line[start+1 : end]
				parts := strings.Fields(content)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}
	return ""
}
