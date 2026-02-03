package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		wantErr bool
	}{
		{
			name:    "valid with key and model",
			apiKey:  "test-key",
			model:   "gemini-2.0-flash",
			wantErr: false,
		},
		{
			name:    "valid with key only",
			apiKey:  "test-key",
			model:   "",
			wantErr: false,
		},
		{
			name:    "empty key",
			apiKey:  "",
			model:   "gemini-2.0-flash",
			wantErr: true,
		},
		{
			name:    "whitespace key",
			apiKey:  "   ",
			model:   "gemini-2.0-flash",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.apiKey, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestClient_SuggestCommits(t *testing.T) {
	// Mock server that returns a valid Gemini response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		// Check API key in query param
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("expected API key in query param")
		}

		// Return mock response
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: `{"suggestions":[{"type":"feat","subject":"add new feature","body":"","footer":""},{"type":"fix","subject":"fix bug","body":"","footer":""},{"type":"docs","subject":"update docs","body":"","footer":""}]}`},
						},
					},
					FinishReason: "STOP",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", "gemini-2.0-flash")
	client.baseURL = server.URL

	input := ports.SuggestInput{
		StagedDiff:  "diff --git a/main.go b/main.go\n+func main() {}",
		FileList:    []string{"main.go"},
		Model:       "gemini-2.0-flash",
		Temperature: 0.7,
	}

	suggestions, err := client.SuggestCommits(context.Background(), input)
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	if len(suggestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(suggestions))
	}

	// Verify first suggestion
	if suggestions[0].Type != "feat" {
		t.Errorf("expected type 'feat', got %q", suggestions[0].Type)
	}
	if suggestions[0].Subject != "add new feature" {
		t.Errorf("expected subject 'add new feature', got %q", suggestions[0].Subject)
	}
}

func TestClient_SuggestCommits_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    400,
				"message": "Invalid request",
				"status":  "INVALID_ARGUMENT",
			},
		})
	}))
	defer server.Close()

	client, _ := NewClient("test-key", "gemini-2.0-flash")
	client.baseURL = server.URL

	input := ports.SuggestInput{
		StagedDiff: "test diff",
		Model:      "gemini-2.0-flash",
	}

	_, err := client.SuggestCommits(context.Background(), input)
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestClient_SuggestCommits_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", "gemini-2.0-flash")
	client.baseURL = server.URL

	input := ports.SuggestInput{
		StagedDiff: "test diff",
		Model:      "gemini-2.0-flash",
	}

	_, err := client.SuggestCommits(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

func TestClient_SuggestCommits_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: "not valid json"},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", "gemini-2.0-flash")
	client.baseURL = server.URL

	input := ports.SuggestInput{
		StagedDiff: "test diff",
		Model:      "gemini-2.0-flash",
	}

	_, err := client.SuggestCommits(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid JSON in response")
	}
}

func TestClient_AnalyzeFileRelations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: `{"isHomogeneous":true,"groups":[{"label":"All changes","files":["file1.go","file2.go"],"confidence":0.9}],"reasoning":"Files are related"}`},
						},
					},
					FinishReason: "STOP",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", "gemini-2.0-flash")
	client.baseURL = server.URL

	input := ports.FileAnalysisInput{
		Files: []ports.StagedFile{
			{Path: "file1.go", Status: "M"},
			{Path: "file2.go", Status: "A"},
		},
		DiffContent: "test diff",
		Model:       "gemini-2.0-flash",
	}

	result, err := client.AnalyzeFileRelations(context.Background(), input)
	if err != nil {
		t.Fatalf("AnalyzeFileRelations failed: %v", err)
	}

	if !result.IsHomogeneous {
		t.Error("expected homogeneous result")
	}

	if len(result.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(result.Groups))
	}

	if result.Groups[0].Label != "All changes" {
		t.Errorf("expected label 'All changes', got %q", result.Groups[0].Label)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON",
			input:    `{"key":"value"}`,
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with markdown code block",
			input:    "```json\n{\"key\":\"value\"}\n```",
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    "Here is the JSON: {\"key\":\"value\"} done",
			expected: `{"key":"value"}`,
		},
		{
			name:     "JSON with whitespace",
			input:    "  \n  {\"key\":\"value\"}  \n  ",
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildCommitPrompt(t *testing.T) {
	diff := "test diff content"
	prompt := buildCommitPrompt(diff)

	if prompt == "" {
		t.Error("prompt should not be empty")
	}

	// Verify diff is included
	if !contains(prompt, diff) {
		t.Error("prompt should contain the diff")
	}

	// Verify it asks for 3 suggestions
	if !contains(prompt, "3") {
		t.Error("prompt should mention 3 suggestions")
	}
}

func TestBuildFileAnalysisPrompt(t *testing.T) {
	input := ports.FileAnalysisInput{
		Files: []ports.StagedFile{
			{Path: "file1.go", Status: "M"},
			{Path: "file2.go", Status: "A"},
		},
		DiffContent: "test diff",
	}

	prompt := buildFileAnalysisPrompt(input)

	if prompt == "" {
		t.Error("prompt should not be empty")
	}

	// Verify files are included
	if !contains(prompt, "file1.go") {
		t.Error("prompt should contain file1.go")
	}
	if !contains(prompt, "file2.go") {
		t.Error("prompt should contain file2.go")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
