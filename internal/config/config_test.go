package config

import (
	"os"
	"testing"
)

func isolateUserConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	prevAppData := os.Getenv("APPDATA")
	prevXDG := os.Getenv("XDG_CONFIG_HOME")
	prevHome := os.Getenv("HOME")

	// os.UserConfigDir consults these on common platforms.
	os.Setenv("APPDATA", dir)
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("HOME", dir)

	t.Cleanup(func() {
		if prevAppData == "" {
			os.Unsetenv("APPDATA")
		} else {
			os.Setenv("APPDATA", prevAppData)
		}
		if prevXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", prevXDG)
		}
		if prevHome == "" {
			os.Unsetenv("HOME")
		} else {
			os.Setenv("HOME", prevHome)
		}
	})
}

func TestConfigLoad(t *testing.T) {
	isolateUserConfigDir(t)

	// Set up test environment with new env var names
	os.Setenv("OPENAI_API_KEY", "sk-test-1234567890abcdefghij")
	os.Setenv("LLM_PROVIDER", "openai")
	os.Setenv("LLM_MODEL", "gpt-4o")
	os.Setenv("LLM_TEMPERATURE", "0.5")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("LLM_PROVIDER")
		os.Unsetenv("LLM_MODEL")
		os.Unsetenv("LLM_TEMPERATURE")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "openai" {
		t.Errorf("Provider = %s, want openai", cfg.Provider)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %s, want gpt-4o", cfg.Model)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", cfg.Temperature)
	}
	if cfg.APIKey != "sk-test-1234567890abcdefghij" {
		t.Errorf("APIKey not set correctly")
	}
}

func TestConfigValidation(t *testing.T) {
	isolateUserConfigDir(t)

	// Make the test deterministic even if the user has env vars set.
	os.Setenv("LLM_PROVIDER", "openai")
	os.Unsetenv("LLM_MODEL")
	os.Unsetenv("LLM_TEMPERATURE")

	// Clear API key env vars
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("GROQ_API_KEY")
	os.Unsetenv("COMMIT_COACH_API_KEY")
	defer func() {
		os.Unsetenv("LLM_PROVIDER")
		os.Unsetenv("LLM_MODEL")
		os.Unsetenv("LLM_TEMPERATURE")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("GROQ_API_KEY")
		os.Unsetenv("COMMIT_COACH_API_KEY")
	}()

	_, err := Load()
	if err == nil {
		t.Error("Load() should fail without API key")
	}
}

func TestConfigLoadAnthropic(t *testing.T) {
	isolateUserConfigDir(t)

	os.Setenv("ANTHROPIC_API_KEY", "anth-test")
	os.Setenv("LLM_PROVIDER", "anthropic")
	os.Setenv("LLM_MODEL", "claude-3-5-haiku-latest")
	defer func() {
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("LLM_PROVIDER")
		os.Unsetenv("LLM_MODEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Fatalf("Provider = %s, want anthropic", cfg.Provider)
	}
	if cfg.APIKey != "anth-test" {
		t.Fatalf("APIKey not set correctly")
	}
}

func TestConfigDefaults(t *testing.T) {
	isolateUserConfigDir(t)

	// Make the test deterministic even if the user has env vars set.
	os.Setenv("LLM_PROVIDER", "openai")
	os.Unsetenv("LLM_MODEL")
	os.Unsetenv("LLM_TEMPERATURE")

	// Set only required var
	os.Setenv("OPENAI_API_KEY", "sk-test")
	defer func() {
		os.Unsetenv("LLM_PROVIDER")
		os.Unsetenv("LLM_MODEL")
		os.Unsetenv("LLM_TEMPERATURE")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("Default model = %s, want gpt-4o-mini", cfg.Model)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Default temperature = %f, want 0.7", cfg.Temperature)
	}
	if cfg.DiffCap != 8192 {
		t.Errorf("Default diff cap = %d, want 8192", cfg.DiffCap)
	}
	if !cfg.ConfirmSend {
		t.Error("Default confirm send should be true")
	}
	if !cfg.Redact {
		t.Error("Default redact should be true")
	}
}
