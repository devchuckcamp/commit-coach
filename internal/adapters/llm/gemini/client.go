package gemini

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

// Client implements ports.LLM for the Google Gemini API.
//
// Docs: https://ai.google.dev/api/rest
//
// Notes:
// - Uses direct HTTP calls (no SDK) to match other provider patterns.
// - API key passed via query parameter.
// - We enforce strict JSON-only response via the prompt and parse it.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// NewClient creates a new Gemini client.
func NewClient(apiKey, model string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("Gemini API key is required")
	}

	if strings.TrimSpace(model) == "" {
		model = "gemini-2.0-flash"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		model:   model,
		http: &http.Client{
			Timeout: 90 * time.Second,
		},
	}, nil
}

// geminiRequest is the request body for Gemini API.
type geminiRequest struct {
	Contents         []geminiContent    `json:"contents"`
	GenerationConfig geminiGenConfig    `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// geminiResponse is the response from Gemini API.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// SuggestCommits generates commit suggestions using Gemini.
func (c *Client) SuggestCommits(ctx context.Context, input ports.SuggestInput) ([]ports.CommitSuggestion, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = c.model
	}

	prompt := buildCommitPrompt(input.StagedDiff)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: prompt}},
				Role:  "user",
			},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:     float64(input.Temperature),
			MaxOutputTokens: 1400,
		},
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: "You are an expert git commit message writer. Return ONLY valid JSON matching the requested schema. No markdown, no extra text."}},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"gemini: non-200 status=%d model=%q temp=%.2f body_len=%d body_snip=%q",
			resp.StatusCode,
			model,
			input.Temperature,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData geminiResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		observability.Logger().Printf(
			"gemini: failed to unmarshal response JSON: %v; body_len=%d body_snip=%q",
			err,
			len(body),
			observability.Snip(observability.RedactForLog(string(body)), 1200),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if respData.Error != nil {
		return nil, fmt.Errorf("gemini API error: %s (code=%d)", respData.Error.Message, respData.Error.Code)
	}

	if len(respData.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	content := ""
	for _, part := range respData.Candidates[0].Content.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			content = text
			break
		}
	}
	if content == "" {
		return nil, fmt.Errorf("gemini returned empty text content")
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
			"gemini: invalid JSON: %v; raw_len=%d raw_snip=%q; json_len=%d json_snip=%q",
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
		model = c.model
	}

	prompt := buildFileAnalysisPrompt(input)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: prompt}},
				Role:  "user",
			},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:     float64(input.Temperature),
			MaxOutputTokens: 1000,
		},
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: "You are an expert at analyzing code changes. Return ONLY valid JSON matching the requested schema. No markdown, no extra text."}},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		observability.Logger().Printf(
			"gemini: file analysis non-200 status=%d model=%q body_snip=%q",
			resp.StatusCode,
			model,
			observability.Snip(string(body), 600),
		)
		return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData geminiResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if respData.Error != nil {
		return nil, fmt.Errorf("gemini API error: %s", respData.Error.Message)
	}

	if len(respData.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	content := ""
	for _, part := range respData.Candidates[0].Content.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			content = text
			break
		}
	}
	if content == "" {
		return nil, fmt.Errorf("gemini returned empty text content")
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
			"gemini: invalid file analysis JSON: %v; content_snip=%q",
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
