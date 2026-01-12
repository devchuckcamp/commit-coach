package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	in := &Config{
		Provider:    "openai",
		APIKey:      "sk-test",
		Model:       "gpt-4o-mini",
		Temperature: 0.7,
		BaseURL:     "",
		OllamaURL:   "http://localhost:11434",
		DiffCap:     8192,
		ConfirmSend: true,
		DryRun:      false,
		Redact:      true,
		UseCache:    true,
	}

	if err := SaveToFile(path, in); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	// On Windows, permission bits are not meaningful in the same way; just ensure file exists.
	_ = st

	out, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if out == nil {
		t.Fatalf("LoadFromFile() = nil, want config")
	}
	if out.Provider == nil || out.Model == nil || out.APIKey == nil {
		t.Fatalf("expected provider/model/apikey fields to be present")
	}
	if *out.Provider != in.Provider || *out.Model != in.Model || *out.APIKey != in.APIKey {
		t.Fatalf("round-trip mismatch: got provider=%q model=%q key=%q", *out.Provider, *out.Model, *out.APIKey)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if cfg != nil {
		t.Fatalf("LoadFromFile() = %#v, want nil", cfg)
	}
}
