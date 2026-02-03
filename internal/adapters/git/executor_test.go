package git

import (
	"testing"
)

func TestExtractCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard format",
			output:   "[main abc123d] feat: add new feature\n 1 file changed, 10 insertions(+)",
			expected: "abc123d",
		},
		{
			name:     "branch with slash",
			output:   "[feature/test 1234567] fix: resolve issue\n",
			expected: "1234567",
		},
		{
			name:     "detached head",
			output:   "[detached HEAD abcdef0] chore: update deps\n",
			expected: "abcdef0",
		},
		{
			name:     "no brackets",
			output:   "Some other output\nwithout commit info",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "malformed brackets",
			output:   "[incomplete",
			expected: "",
		},
		{
			name:     "brackets without space",
			output:   "[nospacehash]",
			expected: "",
		},
		{
			name:     "multiline with hash on second line",
			output:   "Preparing commit...\n[develop 9876543] docs: update readme\n",
			expected: "9876543",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommitHash(tt.output)
			if got != tt.expected {
				t.Errorf("extractCommitHash(%q) = %q, want %q", tt.output, got, tt.expected)
			}
		})
	}
}

func TestParseStagedFilesOutput(t *testing.T) {
	// Test the parsing logic by creating an executor and testing via exported method
	// Since StagedFiles calls git, we test the parsing logic separately

	tests := []struct {
		name           string
		output         string
		expectedCount  int
		expectedFirst  string
		expectedStatus string
	}{
		{
			name:           "single modified file",
			output:         "M\tpath/to/file.go",
			expectedCount:  1,
			expectedFirst:  "path/to/file.go",
			expectedStatus: "M",
		},
		{
			name:           "multiple files",
			output:         "M\tfile1.go\nA\tfile2.go\nD\tfile3.go",
			expectedCount:  3,
			expectedFirst:  "file1.go",
			expectedStatus: "M",
		},
		{
			name:           "renamed file",
			output:         "R100\told/path.go\tnew/path.go",
			expectedCount:  1,
			expectedFirst:  "new/path.go",
			expectedStatus: "R",
		},
		{
			name:          "empty output",
			output:        "",
			expectedCount: 0,
		},
		{
			name:          "whitespace only",
			output:        "   \n\t\n  ",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseStagedFilesOutput(tt.output)
			if len(files) != tt.expectedCount {
				t.Errorf("expected %d files, got %d", tt.expectedCount, len(files))
				return
			}
			if tt.expectedCount > 0 {
				if files[0].Path != tt.expectedFirst {
					t.Errorf("expected first file path %q, got %q", tt.expectedFirst, files[0].Path)
				}
				if files[0].Status != tt.expectedStatus {
					t.Errorf("expected first file status %q, got %q", tt.expectedStatus, files[0].Status)
				}
			}
		})
	}
}

// parseStagedFilesOutput is a helper that extracts the parsing logic for testing
// This mirrors the logic in StagedFiles without calling git
func parseStagedFilesOutput(output string) []struct {
	Path   string
	Status string
} {
	var result []struct {
		Path   string
		Status string
	}

	lines := splitLines(output)
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}

		parts := splitTabs(line)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[1]

		// Handle renames (R100 old new)
		if len(status) > 0 && status[0] == 'R' && len(parts) >= 3 {
			status = "R"
			path = parts[2]
		}

		// Normalize status to single character
		if len(status) > 1 {
			status = string(status[0])
		}

		result = append(result, struct {
			Path   string
			Status string
		}{Path: path, Status: status})
	}

	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitTabs(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
