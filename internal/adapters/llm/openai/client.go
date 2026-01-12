package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/chuckie/commit-coach/internal/observability"
	"github.com/chuckie/commit-coach/internal/ports"
)

// Client implements ports.LLM for OpenAI API.
type Client struct {
	apiKey  string
	baseURL string
	timeout time.Duration
}

// NewClient creates a new OpenAI client.
func NewClient(apiKey, baseURL string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		timeout: 90 * time.Second,
	}, nil
}

// SuggestCommits generates 3 commit suggestions using OpenAI.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	// Create OpenAI client configuration
	config := openai.DefaultConfig(c.apiKey)
	if c.baseURL != "" {
		config.BaseURL = c.baseURL
	}

	client := openai.NewClientWithConfig(config)

	// Build the prompt
	prompt := c.buildPrompt(input)

	// Create completion request
	req := openai.ChatCompletionRequest{
		Model:       input.Model,
		Temperature: input.Temperature,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	// Make request with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	// Parse response
	content := resp.Choices[0].Message.Content
	suggestions, err := c.parseResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	return suggestions, nil
}

// buildPrompt constructs the prompt for OpenAI.
func (c *Client) buildPrompt(input ports.SuggestInput) string {
	return `You are an expert at writing Conventional Commits. Generate exactly 3 commit message suggestions for the following staged changes.

Staged diff:
` + input.StagedDiff + `

Return ONLY a valid JSON array with exactly 3 objects, each with these fields (no extra fields):
{
  "suggestions": [
    {"type": "feat|fix|docs|style|refactor|perf|test|chore", "subject": "...", "body": "...", "footer": "..."}
  ]
}

Rules:
- Subject: max 72 characters, no newlines
- Body: optional multiline explanation
- Footer: optional, use "BREAKING CHANGE: ..." or "Closes #123"
- Keep messages clear and specific to the staged changes

Return ONLY JSON, no markdown code blocks.`
}

// parseResponse extracts suggestions from the JSON response.
func (c *Client) parseResponse(content string) ([]ports.CommitSuggestion, error) {
	// Try to parse as direct JSON first
	var resp struct {
		Suggestions []ports.CommitSuggestion `json:"suggestions"`
	}

	// Remove markdown code blocks if present
	jsonContent := c.extractJSON(content)

	if err := json.Unmarshal([]byte(jsonContent), &resp); err != nil {
		observability.Logger().Printf(
			"openai: invalid JSON: %v; raw_len=%d raw_snip=%q; json_len=%d json_snip=%q",
			err,
			len(content),
			observability.Snip(observability.RedactForLog(content), 600),
			len(jsonContent),
			observability.Snip(observability.RedactForLog(jsonContent), 600),
		)
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	if len(resp.Suggestions) != 3 {
		return nil, fmt.Errorf("expected 3 suggestions, got %d", len(resp.Suggestions))
	}

	return resp.Suggestions, nil
}

// extractJSON extracts JSON from response (handles markdown code blocks).
func (c *Client) extractJSON(content string) string {
	// Remove markdown code fence if present
	if len(content) > 7 && content[:7] == "```json" {
		content = content[7:]
	}
	if len(content) > 3 && content[:3] == "```" {
		content = content[3:]
	}
	if len(content) > 3 && content[len(content)-3:] == "```" {
		content = content[:len(content)-3]
	}
	return content
}
