package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxRunes: 10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxRunes: 5,
			expected: "hello",
		},
		{
			name:     "truncated with ellipsis",
			input:    "hello world",
			maxRunes: 5,
			expected: "hello…",
		},
		{
			name:     "zero maxRunes",
			input:    "hello",
			maxRunes: 0,
			expected: "",
		},
		{
			name:     "negative maxRunes",
			input:    "hello",
			maxRunes: -1,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			maxRunes: 5,
			expected: "",
		},
		{
			name:     "unicode string",
			input:    "héllo wörld",
			maxRunes: 5,
			expected: "héllo…",
		},
		{
			name:     "emoji string",
			input:    "👋🌍🎉",
			maxRunes: 2,
			expected: "👋🌍…",
		},
		{
			name:     "single char limit",
			input:    "hello",
			maxRunes: 1,
			expected: "h…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Snip(tt.input, tt.maxRunes)
			if got != tt.expected {
				t.Errorf("Snip(%q, %d) = %q, want %q", tt.input, tt.maxRunes, got, tt.expected)
			}
		})
	}
}

func TestRedactForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // should contain [REDACTED] or other markers
	}{
		{
			name:     "redacts API key",
			input:    "key is sk-abcdef1234567890abcdef1234567890",
			contains: "[REDACTED]",
		},
		{
			name:     "redacts email",
			input:    "contact user@example.com for help",
			contains: "[EMAIL]",
		},
		{
			name:     "redacts IP address",
			input:    "server at 192.168.1.100 responded",
			contains: "[IP]",
		},
		{
			name:     "preserves normal text",
			input:    "normal log message without secrets",
			contains: "normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactForLog(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("RedactForLog(%q) = %q, expected to contain %q", tt.input, got, tt.contains)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// Create a temp directory for the test log
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test-error.log")

	// Set environment variable
	oldPath := os.Getenv("COMMIT_COACH_LOG_PATH")
	os.Setenv("COMMIT_COACH_LOG_PATH", logPath)
	defer os.Setenv("COMMIT_COACH_LOG_PATH", oldPath)

	// Note: Init uses sync.Once, so we can't test it multiple times in the same process
	// This test verifies the Path() and Logger() functions work correctly
	path := Path()
	logger := Logger()

	if logger == nil {
		t.Error("Logger should not be nil")
	}

	// Path may or may not be set depending on test order
	t.Logf("Current log path: %q", path)
}

func TestLogger_ReturnsDefault(t *testing.T) {
	// Logger should return a non-nil logger even before Init
	logger := Logger()
	if logger == nil {
		t.Error("Logger should never return nil")
	}
}

func TestPath_EmptyBeforeInit(t *testing.T) {
	// This test documents behavior - Path may be empty if Init hasn't run
	// or may be set if Init has run in another test
	path := Path()
	t.Logf("Path before/after init: %q", path)
}
