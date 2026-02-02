package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devchuckcamp/commit-coach/internal/observability"
	"github.com/devchuckcamp/commit-coach/internal/ports"
)

// Client implements ports.LLM for the Anthropic Messages API.
//
// Docs: https://docs.anthropic.com/en/api/messages
//
// Notes:
// - We enforce a strict JSON-only response via the prompt and then parse it.
// - We do not log diffs; logs are redacted/snipped.
// - We require a model in input.Model.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient creates a new Anthropic client.
func NewClient(apiKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.anthropic.com/v1",
		http: &http.Client{
			Timeout: 90 * time.Second,
		},
	}, nil
}

// SuggestCommits generates commit suggestions using Anthropic.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, fmt.Errorf("anthropic model is required")
	}

	prompt := buildCommitPrompt(input.StagedDiff)

	reqBody := map[string]interface{}{
		"model":       model,
		"max_tokens":  1400,
		"temperature": float64(input.Temperature),
		"system":      "You are an expert git commit message writer. Return ONLY valid JSON matching the requested schema. No markdown, no extra text.",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"anthropic: non-200 status=%d model=%q temp=%.2f body_len=%d body_snip=%q",
			resp.StatusCode,
			model,
			input.Temperature,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		observability.Logger().Printf(
			"anthropic: failed to unmarshal response JSON: %v; body_len=%d body_snip=%q",
			err,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	for _, block := range respData.Content {
		if block.Type == "text" {
			content = strings.TrimSpace(block.Text)
			if content != "" {
				break
			}
		}
	}
	if content == "" {
		return nil, fmt.Errorf("anthropic returned empty text content")
	}

	suggestions, err := parseSuggestionsJSON(content)
	if err != nil {
		return nil, err
	}
	if len(suggestions) != 3 {
		return nil, fmt.Errorf("expected 3 suggestions, got %d", len(suggestions))
	}
	return suggestions, nil
}

func buildCommitPrompt(diff string) string {
	return fmt.Sprintf(`Generate exactly 3 Conventional Commit suggestions for this staged diff.

<diff>
%s
</diff>

Return ONLY a single JSON object with this exact shape:
{"suggestions":[{"type":"feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert","subject":"...","body":"...","footer":"..."}]}

Rules:
- Exactly 3 suggestions
- subject: max 72 characters, no newlines
- body/footer may be empty strings
`, diff)
}

func parseSuggestionsJSON(content string) ([]ports.CommitSuggestion, error) {
	var resp struct {
		Suggestions []ports.CommitSuggestion `json:"suggestions"`
	}

	jsonContent := extractJSON(content)

	if err := json.Unmarshal([]byte(jsonContent), &resp); err != nil {
		observability.Logger().Printf(
			"anthropic: invalid JSON: %v; raw_len=%d raw_snip=%q; json_len=%d json_snip=%q",
			err,
			len(content),
			observability.Snip(observability.RedactForLog(content), 600),
			len(jsonContent),
			observability.Snip(observability.RedactForLog(jsonContent), 600),
		)
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	return resp.Suggestions, nil
}

func extractJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	// Best-effort: if the model included any surrounding text, pull out the
	// outermost JSON object.
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return trimmed
}

// AnalyzeFileRelations analyzes if staged files belong in the same commit.
func (c *Client) AnalyzeFileRelations(ctx context.Context, input ports.FileAnalysisInput) (*ports.FileAnalysisResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, fmt.Errorf("anthropic model is required")
	}

	prompt := buildFileAnalysisPrompt(input)

	reqBody := map[string]interface{}{
		"model":       model,
		"max_tokens":  1000,
		"temperature": float64(input.Temperature),
		"system":      "You are an expert at analyzing code changes. Return ONLY valid JSON matching the requested schema. No markdown, no extra text.",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"anthropic: file analysis non-200 status=%d model=%q body_snip=%q",
			resp.StatusCode,
			model,
			observability.Snip(string(body), 600),
		)
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	for _, block := range respData.Content {
		if block.Type == "text" {
			content = strings.TrimSpace(block.Text)
			if content != "" {
				break
			}
		}
	}
	if content == "" {
		return nil, fmt.Errorf("anthropic returned empty text content")
	}

	return parseFileAnalysisJSON(extractJSON(content))
}

func buildFileAnalysisPrompt(input ports.FileAnalysisInput) string {
	var fileList string
	for _, f := range input.Files {
		fileList += fmt.Sprintf("  %s %s\n", f.Status, f.Path)
	}

	return fmt.Sprintf(`Analyze if these staged files belong in the same commit.

<files>
%s</files>

<diff>
%s
</diff>

Return ONLY a single JSON object with this exact shape:
{"isHomogeneous":true/false,"groups":[{"label":"...","files":["path1","path2"],"confidence":0.9}],"reasoning":"..."}

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
			"anthropic: invalid file analysis JSON: %v; content_snip=%q",
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
