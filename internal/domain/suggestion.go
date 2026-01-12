package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ValidCommitTypes is the enumeration of allowed commit types.
var ValidCommitTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "perf", "test", "chore", "build", "ci", "revert",
}

// Suggestion represents a validated commit suggestion.
type Suggestion struct {
	Type    string
	Subject string
	Body    string
	Footer  string
}

// Validate checks a suggestion against domain rules.
func (s Suggestion) Validate() error {
	// Type validation
	if s.Type == "" {
		return fmt.Errorf("type is required")
	}
	if !isValidType(s.Type) {
		return fmt.Errorf("invalid type %q; must be one of: %v", s.Type, ValidCommitTypes)
	}

	// Subject validation
	if s.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if len(s.Subject) > 72 {
		return fmt.Errorf("subject exceeds 72 characters (%d)", len(s.Subject))
	}
	if strings.Contains(s.Subject, "\n") {
		return fmt.Errorf("subject must not contain newlines")
	}
	if hasControlChars(s.Subject) {
		return fmt.Errorf("subject contains control characters")
	}

	// Body validation (optional)
	if s.Body != "" && hasControlChars(s.Body) {
		return fmt.Errorf("body contains control characters")
	}

	// Footer validation (optional)
	if s.Footer != "" {
		if !isValidFooter(s.Footer) {
			return fmt.Errorf("invalid footer format; must match ^(BREAKING CHANGE|Closes|Refs): .*")
		}
		if hasControlChars(s.Footer) {
			return fmt.Errorf("footer contains control characters")
		}
	}

	return nil
}

// Normalize applies whitespace normalization to the suggestion.
func (s *Suggestion) Normalize() {
	s.Type = strings.TrimSpace(strings.ToLower(s.Type))
	s.Subject = strings.TrimSpace(s.Subject)
	s.Body = strings.TrimSpace(s.Body)
	s.Footer = strings.TrimSpace(s.Footer)

	// Truncate subject if needed (though this should not happen after validation)
	if len(s.Subject) > 72 {
		s.Subject = s.Subject[:72]
	}
}

// Format returns the formatted commit message.
func (s Suggestion) Format() string {
	msg := fmt.Sprintf("%s: %s", s.Type, s.Subject)
	if s.Body != "" {
		msg += "\n\n" + s.Body
	}
	if s.Footer != "" {
		msg += "\n\n" + s.Footer
	}
	return msg
}

// isValidType checks if type is in the enumeration.
func isValidType(t string) bool {
	for _, valid := range ValidCommitTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// isValidFooter checks if footer matches expected format.
func isValidFooter(f string) bool {
	pattern := `^(BREAKING CHANGE|Closes|Refs): .+`
	matched, _ := regexp.MatchString(pattern, f)
	return matched
}

// hasControlChars checks for ASCII control characters (0x00-0x1F except newline/tab).
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' {
			return true
		}
		if r == 127 { // DEL
			return true
		}
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return true
		}
	}
	return false
}
