package security

import (
	"regexp"
	"strings"
)

// Redactor implements ports.Redactor with built-in patterns.
type Redactor struct {
	patterns []*regexp.Regexp
}

// NewRedactor creates a new redactor with default patterns.
func NewRedactor() *Redactor {
	patterns := []*regexp.Regexp{
		// OpenAI/Anthropic API keys
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
		// AWS keys
		regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
		// Authorization headers
		regexp.MustCompile(`(?i)(?:authorization|auth|token):\s*Bearer\s+[a-zA-Z0-9._\-]+`),
		// JSON API key patterns
		regexp.MustCompile(`"(?:api_key|apiKey|API_KEY)":\s*"[^"]+"`),
		// Common password patterns
		regexp.MustCompile(`(?i)(?:password|passwd|pwd):\s*"[^"]+"`),
		// Google API keys
		regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
		// GitHub tokens
		regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
		regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
		// Private keys (PEM format start)
		regexp.MustCompile(`-----BEGIN (?:RSA |DSA |EC )?PRIVATE KEY-----`),
	}
	return &Redactor{patterns: patterns}
}

// Redact removes sensitive patterns from text.
func (r *Redactor) Redact(text string) string {
	result := text
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// RedactLog is more aggressive, also removing IP addresses and emails.
func (r *Redactor) RedactLog(text string) string {
	result := r.Redact(text)
	// Redact IP addresses
	ipPattern := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	result = ipPattern.ReplaceAllString(result, "[IP]")
	// Redact email addresses
	emailPattern := regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	result = emailPattern.ReplaceAllString(result, "[EMAIL]")
	return result
}

// Contains checks if text contains any sensitive pattern (for warnings).
func (r *Redactor) Contains(text string) bool {
	for _, pattern := range r.patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// SummarizeRedactions describes what was redacted.
func SummarizeRedactions(original, redacted string) string {
	if original == redacted {
		return "no redactions"
	}
	count := strings.Count(redacted, "[REDACTED]")
	return "removed " + string(rune(count)) + " secret(s)"
}
