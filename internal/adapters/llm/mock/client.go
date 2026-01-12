package mock

import (
	"context"
	"hash/fnv"

	"github.com/chuckie/commit-coach/internal/ports"
)

// Client is a mock LLM implementation for testing purposes.
type Client struct{}

// NewClient creates a new mock LLM client.
func NewClient() *Client {
	return &Client{}
}

// SuggestCommits returns deterministic mock commit suggestions based on the input.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	// Deterministic based on diff content hash
	hash := hashString(input.StagedDiff)

	patterns := []struct {
		commitType string
		subject    string
		body       string
	}{
		{"feat", "add new functionality to enhance user experience", "Introduces new features that improve the user experience and provide additional value."},
		{"fix", "resolve issue affecting system stability", "Addresses a defect that was impacting stability and improves overall reliability."},
		{"refactor", "simplify internal logic and improve code clarity", "Refactors internal implementation to improve maintainability and reduce complexity."},
		{"docs", "update documentation for recent changes", "Updates documentation to reflect the latest behavior and usage patterns."},
		{"chore", "update dependencies and maintenance tasks", "Performs routine maintenance and dependency updates to keep the project healthy."},
	}

	result := make([]ports.CommitSuggestion, 0, 3)
	for i := 0; i < 3; i++ {
		idx := int((hash + uint64(i)) % uint64(len(patterns)))
		p := patterns[idx]
		subject := p.subject
		if len(subject) > 72 {
			subject = subject[:72]
		}
		result = append(result, ports.CommitSuggestion{
			Type:    p.commitType,
			Subject: subject,
			Body:    p.body,
			Footer:  "",
		})
	}

	return result, nil
}

// hashString computes a simple hash of a string for deterministic behavior.
func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

