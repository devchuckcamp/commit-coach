package security

import (
	"strings"
	"testing"
)

func TestRedactor(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		name     string
		input    string
		contains string
		redacted bool
	}{
		{
			name:     "redact openai key",
			input:    `"api_key": "sk-proj-1234567890abcdefghij"`,
			contains: "sk-",
			redacted: true,
		},
		{
			name:     "redact authorization header",
			input:    `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
			contains: "Bearer",
			redacted: true,
		},
		{
			name:     "redact aws key",
			input:    `AKIA1234567890ABCDEF`,
			contains: "AKIA",
			redacted: true,
		},
		{
			name:     "preserve normal code",
			input:    `func apiHandler(w http.ResponseWriter, r *http.Request) {}`,
			contains: "apiHandler",
			redacted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			hasRedaction := strings.Contains(result, "[REDACTED]")
			if hasRedaction != tt.redacted {
				t.Errorf("Redaction mismatch: input=%q, result=%q, wantRedacted=%v", tt.input, result, tt.redacted)
			}
			if !tt.redacted && !strings.Contains(result, tt.contains) {
				t.Errorf("Expected string not found in result: %q not in %q", tt.contains, result)
			}
		})
	}
}

func TestRedactorLog(t *testing.T) {
	r := NewRedactor()

	input := `Email: john@example.com, IP: 192.168.1.1, Key: sk-1234567890abcdefghij`
	result := r.RedactLog(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Error("Expected secrets to be redacted")
	}
	if !strings.Contains(result, "[EMAIL]") {
		t.Error("Expected email to be redacted to [EMAIL]")
	}
	if !strings.Contains(result, "[IP]") {
		t.Error("Expected IP to be redacted to [IP]")
	}
}

func TestRedactorContains(t *testing.T) {
	r := NewRedactor()

	if !r.Contains("sk-1234567890abcdefghijk") {
		t.Error("Should detect API key")
	}
	if r.Contains("normal code and text") {
		t.Error("Should not flag normal code")
	}
}
