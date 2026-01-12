package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chuckie/commit-coach/internal/observability"
	"github.com/chuckie/commit-coach/internal/ports"
)

// Client implements ports.LLM for Groq API (OpenAI-compatible).
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// NewClient creates a new Groq client.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = "mixtral-8x7b-32768"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.groq.com/openai/v1",
		model:   model,
		http: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// SuggestCommits generates commit suggestions using Groq API.
// Groq API is OpenAI-compatible.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	prompt := buildCommitPrompt(input.StagedDiff)

	// JSON-enforced mode works best with low temperature.
	temp := input.Temperature
	if temp > 0.2 {
		temp = 0.2
	}

	reqBody := map[string]interface{}{
		"model": c.model,
		// Ask the OpenAI-compatible API to return a JSON object. Some models may
		// otherwise emit reasoning-only output with empty message.content.
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert git commit message writer. Return ONLY valid JSON matching the requested schema. No markdown, no extra text.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": temp,
		"max_tokens":  1400,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Groq API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"groq: non-200 status=%d model=%q temp=%.2f max_tokens=%v body_len=%d body_snip=%q",
			resp.StatusCode,
			c.model,
			temp,
			reqBody["max_tokens"],
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)

		// Some models/endpoints reject strict JSON mode with "json_validate_failed".
		// Retry once without response_format (but still with a strict prompt) so we
		// can parse JSON from content.
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "\"code\":\"json_validate_failed\"") {
			return c.retryWithoutJSONMode(ctx, input, prompt)
		}

		return nil, fmt.Errorf("groq returned status %d: %s", resp.StatusCode, string(body))
	}

	if len(body) == 0 {
		observability.Logger().Printf("groq: empty HTTP body (status=200) model=%q", c.model)
		return nil, fmt.Errorf("groq returned empty response body")
	}

	var respData struct {
		Choices []struct {
			Message struct {
				Role    string  `json:"role"`
				Content *string `json:"content"`
				Reasoning *string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		observability.Logger().Printf(
			"groq: failed to unmarshal response JSON: %v; body_len=%d body_snip=%q",
			err,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(respData.Choices) == 0 {
		observability.Logger().Printf(
			"groq: no choices in response; body_len=%d body_snip=%q",
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("no choices in response")
	}

	msg := respData.Choices[0].Message
	content := ""
	if msg.Content != nil {
		content = strings.TrimSpace(*msg.Content)
	}
	if content == "" && msg.Reasoning != nil {
		// Some Groq models return reasoning but leave content empty.
		// We'll attempt to parse JSON from reasoning as a fallback.
		content = strings.TrimSpace(*msg.Reasoning)
	}
	if content == "" {
		observability.Logger().Printf(
			"groq: empty assistant output; role=%q body_len=%d body_snip=%q",
			msg.Role,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("groq returned empty assistant output")
	}

	suggestions, err := parseSuggestionsJSON(content)
	if err != nil {
		return nil, err
	}
	if len(suggestions) < 3 {
		return nil, fmt.Errorf("expected 3 suggestions, got %d", len(suggestions))
	}
	return suggestions[:3], nil
}

func (c *Client) retryWithoutJSONMode(ctx context.Context, input ports.SuggestInput, prompt string) ([]ports.CommitSuggestion, error) {
	// Keep it deterministic.
	temp := input.Temperature
	if temp > 0.2 {
		temp = 0.2
	}

	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "Return ONLY valid JSON for the requested schema. Output must start with '{' and end with '}'. No markdown, no extra text.",
			},
			{
				"role": "user",
				"content": prompt,
			},
		},
		"temperature": temp,
		"max_tokens":  1600,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create retry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Groq API (retry): %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"groq: retry non-200 status=%d model=%q temp=%.2f max_tokens=%v body_len=%d body_snip=%q",
			resp.StatusCode,
			c.model,
			temp,
			reqBody["max_tokens"],
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("groq returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Choices []struct {
			Message struct {
				Role    string  `json:"role"`
				Content *string `json:"content"`
				Reasoning *string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &respData); err != nil {
		observability.Logger().Printf(
			"groq: retry failed to unmarshal response JSON: %v; body_len=%d body_snip=%q",
			err,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("failed to parse response (retry): %w", err)
	}
	if len(respData.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response (retry)")
	}

	msg := respData.Choices[0].Message
	content := ""
	if msg.Content != nil {
		content = strings.TrimSpace(*msg.Content)
	}
	if content == "" && msg.Reasoning != nil {
		content = strings.TrimSpace(*msg.Reasoning)
	}
	if content == "" {
		return nil, fmt.Errorf("groq returned empty assistant output (retry)")
	}

	suggestions, err := parseSuggestionsJSON(content)
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
			"groq: invalid JSON: %v; raw_len=%d raw_snip=%q; json_len=%d json_snip=%q",
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
	trimmed = strings.TrimSpace(trimmed)

	// If there's leading/trailing chatter, try to extract the first complete JSON object.
	if obj, ok := firstJSONObject(trimmed); ok {
		return obj
	}
	return trimmed
}

// firstJSONObject returns the first complete JSON object found in s.
// It uses a simple brace-balancing scan and ignores braces inside strings.
func firstJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		b := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}

		if b == '"' {
			inString = true
			continue
		}
		if b == '{' {
			depth++
			continue
		}
		if b == '}' {
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1]), true
			}
		}
	}

	return "", false
}


