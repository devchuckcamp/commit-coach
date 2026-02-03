package mock

import (
	"context"
	"testing"

	"github.com/devchuckcamp/commit-coach/internal/ports"
)

func TestClient_SuggestCommits(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	input := ports.SuggestInput{
		StagedDiff:  "diff --git a/main.go b/main.go\n+func main() {}",
		FileList:    []string{"main.go"},
		Model:       "mock",
		Temperature: 0.7,
	}

	suggestions, err := client.SuggestCommits(ctx, input)
	if err != nil {
		t.Fatalf("SuggestCommits failed: %v", err)
	}

	if len(suggestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(suggestions))
	}

	// Verify all suggestions have valid commit types
	validTypes := map[string]bool{
		"feat": true, "fix": true, "refactor": true, "docs": true, "chore": true,
	}

	for i, s := range suggestions {
		if !validTypes[s.Type] {
			t.Errorf("suggestion %d has invalid type %q", i, s.Type)
		}
		if s.Subject == "" {
			t.Errorf("suggestion %d has empty subject", i)
		}
		if len(s.Subject) > 72 {
			t.Errorf("suggestion %d subject exceeds 72 chars: %d", i, len(s.Subject))
		}
	}
}

func TestClient_SuggestCommits_Deterministic(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	input := ports.SuggestInput{
		StagedDiff: "same diff content",
	}

	// Same input should produce same output
	result1, _ := client.SuggestCommits(ctx, input)
	result2, _ := client.SuggestCommits(ctx, input)

	if len(result1) != len(result2) {
		t.Fatal("results should have same length")
	}

	for i := range result1 {
		if result1[i].Type != result2[i].Type {
			t.Errorf("suggestion %d type mismatch: %q vs %q", i, result1[i].Type, result2[i].Type)
		}
		if result1[i].Subject != result2[i].Subject {
			t.Errorf("suggestion %d subject mismatch", i)
		}
	}
}

func TestClient_SuggestCommits_DifferentInputs(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	input1 := ports.SuggestInput{StagedDiff: "diff content A"}
	input2 := ports.SuggestInput{StagedDiff: "diff content B"}

	result1, _ := client.SuggestCommits(ctx, input1)
	result2, _ := client.SuggestCommits(ctx, input2)

	// Different inputs may produce different outputs
	// Just verify both return valid results
	if len(result1) != 3 || len(result2) != 3 {
		t.Error("both should return 3 suggestions")
	}
}

func TestClient_AnalyzeFileRelations_Homogeneous(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	// 3 or fewer files should be homogeneous
	input := ports.FileAnalysisInput{
		Files: []ports.StagedFile{
			{Path: "file1.go", Status: "M"},
			{Path: "file2.go", Status: "A"},
			{Path: "file3.go", Status: "M"},
		},
	}

	result, err := client.AnalyzeFileRelations(ctx, input)
	if err != nil {
		t.Fatalf("AnalyzeFileRelations failed: %v", err)
	}

	if !result.IsHomogeneous {
		t.Error("expected homogeneous result for 3 files")
	}

	if len(result.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(result.Groups))
	}

	if len(result.Groups[0].Files) != 3 {
		t.Errorf("expected 3 files in group, got %d", len(result.Groups[0].Files))
	}
}

func TestClient_AnalyzeFileRelations_NonHomogeneous(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	// More than 3 files should be non-homogeneous
	input := ports.FileAnalysisInput{
		Files: []ports.StagedFile{
			{Path: "file1.go", Status: "M"},
			{Path: "file2.go", Status: "A"},
			{Path: "file3.go", Status: "M"},
			{Path: "file4.go", Status: "D"},
		},
	}

	result, err := client.AnalyzeFileRelations(ctx, input)
	if err != nil {
		t.Fatalf("AnalyzeFileRelations failed: %v", err)
	}

	if result.IsHomogeneous {
		t.Error("expected non-homogeneous result for 4 files")
	}

	if len(result.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Groups))
	}

	// Verify all files are represented
	totalFiles := 0
	for _, g := range result.Groups {
		totalFiles += len(g.Files)
	}
	if totalFiles != 4 {
		t.Errorf("expected 4 total files across groups, got %d", totalFiles)
	}
}

func TestClient_AnalyzeFileRelations_EmptyFiles(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	input := ports.FileAnalysisInput{
		Files: []ports.StagedFile{},
	}

	result, err := client.AnalyzeFileRelations(ctx, input)
	if err != nil {
		t.Fatalf("AnalyzeFileRelations failed: %v", err)
	}

	if !result.IsHomogeneous {
		t.Error("empty files should be homogeneous")
	}
}

func TestHashString(t *testing.T) {
	// Same string should produce same hash
	h1 := hashString("test")
	h2 := hashString("test")
	if h1 != h2 {
		t.Error("same string should produce same hash")
	}

	// Different strings should produce different hashes
	h3 := hashString("different")
	if h1 == h3 {
		t.Error("different strings should produce different hashes")
	}

	// Empty string should not panic
	h4 := hashString("")
	if h4 == 0 {
		// FNV hash of empty string is not 0, but let's just verify it doesn't panic
		t.Log("empty string hash:", h4)
	}
}
