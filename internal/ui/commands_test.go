package ui

import (
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/domain"
)

func TestParseEditedMessage(t *testing.T) {
	m := &Model{}

	tests := []struct {
		name           string
		input          string
		expectedType   string
		expectedSubj   string
		expectedBody   string
		expectedFooter string
		expectNil      bool
	}{
		{
			name:         "empty string",
			input:        "",
			expectNil:    true,
		},
		{
			name:         "whitespace only",
			input:        "   \n\t  ",
			expectNil:    true,
		},
		{
			name:         "simple type: subject",
			input:        "feat: add new feature",
			expectedType: "feat",
			expectedSubj: "add new feature",
		},
		{
			name:         "fix type",
			input:        "fix: resolve bug in parser",
			expectedType: "fix",
			expectedSubj: "resolve bug in parser",
		},
		{
			name:         "docs type",
			input:        "docs: update README",
			expectedType: "docs",
			expectedSubj: "update README",
		},
		{
			name:         "no colon - fallback to fix",
			input:        "just a subject without type",
			expectedType: "fix",
			expectedSubj: "just a subject without type",
		},
		{
			name:         "invalid type - fallback to fix",
			input:        "invalid: some subject",
			expectedType: "fix",
			expectedSubj: "invalid: some subject",
		},
		{
			name:         "with body",
			input:        "feat: add feature\n\nThis is the body explaining the change.",
			expectedType: "feat",
			expectedSubj: "add feature",
			expectedBody: "This is the body explaining the change.",
		},
		{
			name:         "with footer",
			input:        "feat: add feature\n\nBody text here.\n\nBREAKING CHANGE: API changed",
			expectedType: "feat",
			expectedSubj: "add feature",
			expectedBody: "Body text here.",
			expectedFooter: "BREAKING CHANGE: API changed",
		},
		{
			name:         "closes footer",
			input:        "fix: resolve issue\n\nFixed the problem.\n\nCloses: #123",
			expectedType: "fix",
			expectedSubj: "resolve issue",
			expectedBody: "Fixed the problem.",
			expectedFooter: "Closes: #123",
		},
		{
			name:         "refs footer",
			input:        "docs: update docs\n\nRefs: #456",
			expectedType: "docs",
			expectedSubj: "update docs",
			expectedFooter: "Refs: #456",
		},
		{
			name:         "all valid commit types",
			input:        "refactor: improve code structure",
			expectedType: "refactor",
			expectedSubj: "improve code structure",
		},
		{
			name:         "chore type",
			input:        "chore: update dependencies",
			expectedType: "chore",
			expectedSubj: "update dependencies",
		},
		{
			name:         "test type",
			input:        "test: add unit tests",
			expectedType: "test",
			expectedSubj: "add unit tests",
		},
		{
			name:         "perf type",
			input:        "perf: optimize algorithm",
			expectedType: "perf",
			expectedSubj: "optimize algorithm",
		},
		{
			name:         "style type",
			input:        "style: format code",
			expectedType: "style",
			expectedSubj: "format code",
		},
		{
			name:         "build type",
			input:        "build: update build config",
			expectedType: "build",
			expectedSubj: "update build config",
		},
		{
			name:         "ci type",
			input:        "ci: update CI config",
			expectedType: "ci",
			expectedSubj: "update CI config",
		},
		{
			name:         "revert type",
			input:        "revert: undo previous change",
			expectedType: "revert",
			expectedSubj: "undo previous change",
		},
		{
			name:         "multiple body paragraphs",
			input:        "feat: add feature\n\nFirst paragraph.\n\nSecond paragraph.",
			expectedType: "feat",
			expectedSubj: "add feature",
			expectedBody: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name:         "colon in subject",
			input:        "feat: add feature: with colon",
			expectedType: "feat",
			expectedSubj: "add feature: with colon",
		},
		{
			name:         "extra whitespace",
			input:        "  feat:   add feature  ",
			expectedType: "feat",
			expectedSubj: "add feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.parseEditedMessage(tt.input)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Type != tt.expectedType {
				t.Errorf("type: got %q, want %q", result.Type, tt.expectedType)
			}

			if result.Subject != tt.expectedSubj {
				t.Errorf("subject: got %q, want %q", result.Subject, tt.expectedSubj)
			}

			if result.Body != tt.expectedBody {
				t.Errorf("body: got %q, want %q", result.Body, tt.expectedBody)
			}

			if result.Footer != tt.expectedFooter {
				t.Errorf("footer: got %q, want %q", result.Footer, tt.expectedFooter)
			}
		})
	}
}

func TestParseEditedMessage_ValidCommitTypes(t *testing.T) {
	m := &Model{}

	// Verify all valid commit types are recognized
	for _, validType := range domain.ValidCommitTypes {
		input := validType + ": test subject"
		result := m.parseEditedMessage(input)

		if result == nil {
			t.Errorf("failed to parse valid type %q", validType)
			continue
		}

		if result.Type != validType {
			t.Errorf("type %q was not preserved, got %q", validType, result.Type)
		}
	}
}
