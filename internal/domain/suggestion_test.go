package domain

import (
	"testing"
)

func TestSuggestionValidation(t *testing.T) {
	tests := []struct {
		name    string
		sugg    Suggestion
		wantErr bool
	}{
		{
			name: "valid feat suggestion",
			sugg: Suggestion{
				Type:    "feat",
				Subject: "add new feature",
				Body:    "",
				Footer:  "",
			},
			wantErr: false,
		},
		{
			name: "subject too long",
			sugg: Suggestion{
				Type:    "feat",
				Subject: "this is a very long subject that exceeds the maximum limit of 72 characters",
				Body:    "",
				Footer:  "",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			sugg: Suggestion{
				Type:    "invalid",
				Subject: "some change",
				Body:    "",
				Footer:  "",
			},
			wantErr: true,
		},
		{
			name: "empty subject",
			sugg: Suggestion{
				Type:    "fix",
				Subject: "",
				Body:    "",
				Footer:  "",
			},
			wantErr: true,
		},
		{
			name: "valid with body",
			sugg: Suggestion{
				Type:    "fix",
				Subject: "correct bug in parser",
				Body:    "The parser was incorrectly handling\nmultiline inputs.",
				Footer:  "",
			},
			wantErr: false,
		},
		{
			name: "valid with breaking change",
			sugg: Suggestion{
				Type:    "refactor",
				Subject: "restructure API",
				Body:    "",
				Footer:  "BREAKING CHANGE: old API removed",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sugg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSuggestionNormalize(t *testing.T) {
	sugg := Suggestion{
		Type:    " FEAT ",
		Subject: "  add feature  ",
		Body:    "  some body  ",
		Footer:  "  BREAKING CHANGE: details  ",
	}

	sugg.Normalize()

	if sugg.Type != "feat" {
		t.Errorf("Type not lowercased: %s", sugg.Type)
	}
	if sugg.Subject != "add feature" {
		t.Errorf("Subject not trimmed: %s", sugg.Subject)
	}
	if sugg.Body != "some body" {
		t.Errorf("Body not trimmed: %s", sugg.Body)
	}
	if sugg.Footer != "BREAKING CHANGE: details" {
		t.Errorf("Footer not trimmed: %s", sugg.Footer)
	}
}

func TestSuggestionFormat(t *testing.T) {
	sugg := Suggestion{
		Type:    "fix",
		Subject: "handle nil pointer",
		Body:    "Added null check",
		Footer:  "Closes #123",
	}

	msg := sugg.Format()
	if msg != "fix: handle nil pointer\n\nAdded null check\n\nCloses #123" {
		t.Errorf("Format output incorrect: %q", msg)
	}
}

func TestControlCharsDetection(t *testing.T) {
	tests := []struct {
		input    string
		hasCtrl  bool
	}{
		{"normal text", false},
		{"text\x00with null", true},
		{"text\nwith newline", false},
		{"text\twith tab", false},
		{"text\x1fwith control", true},
	}

	for _, tt := range tests {
		result := hasControlChars(tt.input)
		if result != tt.hasCtrl {
			t.Errorf("hasControlChars(%q) = %v, want %v", tt.input, result, tt.hasCtrl)
		}
	}
}
