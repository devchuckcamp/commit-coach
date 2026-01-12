package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	Provider    string
	APIKey      string
	Model       string
	Temperature float32
	BaseURL     string
	OllamaURL   string
	DiffCap     int
	ConfirmSend bool
	DryRun      bool
	Redact      bool
	UseCache    bool
}

// Load loads configuration with precedence:
// environment variables → config file → defaults.
func Load() (*Config, error) {
	// 1) Defaults
	cfg := &Config{
		Provider:    "openai",
		APIKey:      "",
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

	// 2) Config file (best-effort)
	if path, err := DefaultConfigPath(); err == nil {
		if fileCfg, err := LoadFromFile(path); err == nil && fileCfg != nil {
			applyPartialConfig(cfg, fileCfg)
		}
	}

	// 3) Env overrides
	if v, ok := os.LookupEnv("LLM_PROVIDER"); ok && v != "" {
		cfg.Provider = v
	}
	if v, ok := os.LookupEnv("LLM_MODEL"); ok && v != "" {
		cfg.Model = v
	}
	if v, ok := os.LookupEnv("OPENAI_BASE_URL"); ok {
		cfg.BaseURL = v
	}
	if v, ok := os.LookupEnv("OLLAMA_URL"); ok && v != "" {
		cfg.OllamaURL = v
	}
	if v, ok := os.LookupEnv("LLM_TEMPERATURE"); ok && v != "" {
		cfg.Temperature = getEnvFloat("LLM_TEMPERATURE", cfg.Temperature)
	}
	if _, ok := os.LookupEnv("DIFF_CAP_BYTES"); ok {
		cfg.DiffCap = getEnvInt("DIFF_CAP_BYTES", cfg.DiffCap)
	}
	if _, ok := os.LookupEnv("CONFIRM_BEFORE_SEND"); ok {
		cfg.ConfirmSend = getEnvBool("CONFIRM_BEFORE_SEND", cfg.ConfirmSend)
	}
	if _, ok := os.LookupEnv("DRY_RUN"); ok {
		cfg.DryRun = getEnvBool("DRY_RUN", cfg.DryRun)
	}
	if _, ok := os.LookupEnv("REDACT_SECRETS"); ok {
		cfg.Redact = getEnvBool("REDACT_SECRETS", cfg.Redact)
	}
	if _, ok := os.LookupEnv("ENABLE_CACHE"); ok {
		cfg.UseCache = getEnvBool("ENABLE_CACHE", cfg.UseCache)
	}

	// Provider-specific API keys:
	// - If env var exists (even empty), it wins.
	// - Else we keep any value loaded from config file.
	switch cfg.Provider {
	case "openai":
		if _, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
			cfg.APIKey = getEnv("OPENAI_API_KEY", "")
		}
	case "anthropic":
		if _, ok := os.LookupEnv("ANTHROPIC_API_KEY"); ok {
			cfg.APIKey = getEnv("ANTHROPIC_API_KEY", "")
		}
	case "groq":
		if _, ok := os.LookupEnv("GROQ_API_KEY"); ok {
			cfg.APIKey = getEnv("GROQ_API_KEY", "")
		}
	case "mock":
		cfg.APIKey = "mock"
	case "ollama":
		cfg.APIKey = "ollama"
	}

	// Validate
	if cfg.Provider != "openai" && cfg.Provider != "anthropic" && cfg.Provider != "groq" && cfg.Provider != "mock" && cfg.Provider != "ollama" {
		return nil, fmt.Errorf("invalid provider: %s (must be 'openai', 'anthropic', 'groq', 'mock', or 'ollama')", cfg.Provider)
	}

	if (cfg.Provider == "openai" || cfg.Provider == "groq" || cfg.Provider == "anthropic") && cfg.APIKey == "" {
		// Anthropic uses ANTHROPIC_API_KEY (not PROVIDER_API_KEY like openai/groq), so keep the hint explicit.
		if cfg.Provider == "anthropic" {
			return cfg, fmt.Errorf("%w: API key not found for provider anthropic; set ANTHROPIC_API_KEY env var", ErrSetupRequired)
		}
		return cfg, fmt.Errorf("%w: API key not found for provider %s; set %s_API_KEY env var", ErrSetupRequired, cfg.Provider, strings.ToUpper(cfg.Provider))
	}

	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		return nil, fmt.Errorf("temperature must be between 0 and 2, got %.2f", cfg.Temperature)
	}

	if cfg.DiffCap <= 0 {
		return nil, fmt.Errorf("diff cap must be positive, got %d", cfg.DiffCap)
	}

	return cfg, nil
}

func applyPartialConfig(dst *Config, src *PartialConfig) {
	if dst == nil || src == nil {
		return
	}
	if src.Provider != nil {
		dst.Provider = *src.Provider
	}
	if src.APIKey != nil {
		dst.APIKey = *src.APIKey
	}
	if src.Model != nil {
		dst.Model = *src.Model
	}
	if src.Temperature != nil {
		dst.Temperature = *src.Temperature
	}
	if src.BaseURL != nil {
		dst.BaseURL = *src.BaseURL
	}
	if src.OllamaURL != nil {
		dst.OllamaURL = *src.OllamaURL
	}
	if src.DiffCap != nil {
		dst.DiffCap = *src.DiffCap
	}
	if src.ConfirmSend != nil {
		dst.ConfirmSend = *src.ConfirmSend
	}
	if src.DryRun != nil {
		dst.DryRun = *src.DryRun
	}
	if src.Redact != nil {
		dst.Redact = *src.Redact
	}
	if src.UseCache != nil {
		dst.UseCache = *src.UseCache
	}
}

// IsSetupRequired returns true when err indicates we should prompt for config.
func IsSetupRequired(err error) bool {
	return errors.Is(err, ErrSetupRequired)
}

// getEnv retrieves an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as int with a default value.
func getEnvInt(key string, defaultValue int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

// getEnvFloat retrieves an environment variable as float32 with a default value.
func getEnvFloat(key string, defaultValue float32) float32 {
	if val, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(val, 32); err == nil {
			return float32(f)
		}
	}
	return defaultValue
}

// getEnvBool retrieves an environment variable as bool with a default value.
func getEnvBool(key string, defaultValue bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultValue
}
