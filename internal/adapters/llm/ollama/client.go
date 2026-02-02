package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/chuckie/commit-coach/internal/observability"
	"github.com/chuckie/commit-coach/internal/ports"
)

// Client is an Ollama LLM client for local inference.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewClient creates a new Ollama client.
func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama2"
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: 0}, // Let context handle timeout
	}
}

// SuggestCommits generates commit suggestions using Ollama.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	// Build prompt
	prompt := buildCommitPrompt(input.StagedDiff)

	// Call Ollama API
	reqBody := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": input.Temperature,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var respData struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	suggestions, err := parseSuggestionsJSON(respData.Response)
	if err != nil {
		return nil, err
	}
	if len(suggestions) < 3 {
		return nil, fmt.Errorf("expected 3 suggestions, got %d", len(suggestions))
	}
	return suggestions[:3], nil
}

// buildCommitPrompt creates a prompt for commit message generation.
func buildCommitPrompt(diff string) string {
	return fmt.Sprintf(`You are an expert at writing Conventional Commits.

Generate exactly 3 commit message suggestions for the following staged diff.

<diff>
%s
</diff>

Return ONLY valid JSON (no markdown code blocks) with this shape:
{
  "suggestions": [
    {"type": "feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert", "subject": "...", "body": "...", "footer": "..."}
  ]
}

Rules:
- Exactly 3 suggestions
- subject: max 72 characters, no newlines
- body/footer optional
`, diff)
}

func parseSuggestionsJSON(content string) ([]ports.CommitSuggestion, error) {
	var resp struct {
		Suggestions []ports.CommitSuggestion `json:"suggestions"`
	}

	jsonContent := extractJSON(content)
	if err := json.Unmarshal([]byte(jsonContent), &resp); err != nil {
		observability.Logger().Printf(
			"ollama: invalid JSON: %v; raw_len=%d raw_snip=%q; json_len=%d json_snip=%q",
			err,
			len(content),
			observability.Snip(observability.RedactForLog(content), 600),
			len(jsonContent),
			observability.Snip(observability.RedactForLog(jsonContent), 600),
		)
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(resp.Suggestions) == 0 {
		return nil, errors.New("no suggestions in response")
	}
	return resp.Suggestions, nil
}

func extractJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

// AnalyzeFileRelations analyzes if staged files belong in the same commit.
func (c *Client) AnalyzeFileRelations(ctx context.Context, input ports.FileAnalysisInput) (*ports.FileAnalysisResult, error) {
	prompt := buildFileAnalysisPrompt(input)

	reqBody := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": input.Temperature,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return parseFileAnalysisJSON(extractJSON(respData.Response))
}

func buildFileAnalysisPrompt(input ports.FileAnalysisInput) string {
	var fileList string
	for _, f := range input.Files {
		fileList += fmt.Sprintf("  %s %s\n", f.Status, f.Path)
	}

	return fmt.Sprintf(`You are an expert at analyzing code changes.

Analyze if these staged files belong in the same commit.

<files>
%s</files>

<diff>
%s
</diff>

Return ONLY valid JSON (no markdown code blocks) with this shape:
{
  "isHomogeneous": true/false,
  "groups": [
    {"label": "group description", "files": ["path1", "path2"], "confidence": 0.9}
  ],
  "reasoning": "brief explanation"
}

Rules:
- isHomogeneous: true if all files are logically related for a single commit
- groups: logical groupings of related files (at least 1 group)
- confidence: 0.0-1.0
- Be conservative: if files could reasonably go together, mark as homogeneous
`, fileList, input.DiffContent)
}

func parseFileAnalysisJSON(content string) (*ports.FileAnalysisResult, error) {
	var resp struct {
		IsHomogeneous bool `json:"isHomogeneous"`
		Groups        []struct {
			Label      string   `json:"label"`
			Files      []string `json:"files"`
			Confidence float32  `json:"confidence"`
		} `json:"groups"`
		Reasoning string `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		observability.Logger().Printf(
			"ollama: invalid file analysis JSON: %v; content_snip=%q",
			err,
			observability.Snip(content, 600),
		)
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	result := &ports.FileAnalysisResult{
		IsHomogeneous: resp.IsHomogeneous,
		Reasoning:     resp.Reasoning,
	}

	for _, g := range resp.Groups {
		result.Groups = append(result.Groups, ports.FileGroup{
			Label:      g.Label,
			Files:      g.Files,
			Confidence: g.Confidence,
		})
	}

	return result, nil
}

