package mock

import (
	"context"
	"hash/fnv"

	"github.com/devchuckcamp/commit-coach/internal/ports"
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

// AnalyzeFileRelations returns deterministic mock file analysis results.
func (c *Client) AnalyzeFileRelations(ctx context.Context, input ports.FileAnalysisInput) (*ports.FileAnalysisResult, error) {
	// Deterministic based on file count
	// If more than 3 files, mark as non-homogeneous for testing purposes
	isHomogeneous := len(input.Files) <= 3

	// Build a single group with all files
	files := make([]string, len(input.Files))
	for i, f := range input.Files {
		files[i] = f.Path
	}

	result := &ports.FileAnalysisResult{
		IsHomogeneous: isHomogeneous,
		Groups: []ports.FileGroup{
			{
				Label:      "All staged changes",
				Files:      files,
				Confidence: 0.85,
			},
		},
		Reasoning: "Mock analysis: files appear related based on staging context.",
	}

	// If not homogeneous, create multiple groups for testing
	if !isHomogeneous && len(input.Files) > 1 {
		mid := len(input.Files) / 2
		result.Groups = []ports.FileGroup{
			{
				Label:      "Group 1",
				Files:      files[:mid],
				Confidence: 0.8,
			},
			{
				Label:      "Group 2",
				Files:      files[mid:],
				Confidence: 0.75,
			},
		}
		result.Reasoning = "Mock analysis: files appear to belong to different logical changes."
	}

	return result, nil
}

