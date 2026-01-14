package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PartialConfig represents a config file with optional fields.
// This prevents missing keys from clobbering defaults.
type PartialConfig struct {
	Provider    *string  `json:"Provider,omitempty"`
	APIKey      *string  `json:"APIKey,omitempty"`
	Model       *string  `json:"Model,omitempty"`
	Temperature *float32 `json:"Temperature,omitempty"`
	BaseURL     *string  `json:"BaseURL,omitempty"`
	OllamaURL   *string  `json:"OllamaURL,omitempty"`
	DiffCap     *int     `json:"DiffCap,omitempty"`
	ConfirmSend *bool    `json:"ConfirmSend,omitempty"`
	DryRun      *bool    `json:"DryRun,omitempty"`
	Redact      *bool    `json:"Redact,omitempty"`
	UseCache    *bool    `json:"UseCache,omitempty"`
}

// DefaultConfigPath returns the default per-user config path.
//
// Typically:
// - Linux:   ~/.config/commit-coach/config.json
// - macOS:   ~/Library/Application Support/commit-coach/config.json
// - Windows: %AppData%/commit-coach/config.json
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(dir, "commit-coach", "config.json"), nil
}

// LoadFromFile loads config from a JSON file. If the file doesn't exist, returns (nil, nil).
func LoadFromFile(path string) (*PartialConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg PartialConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}
	return &cfg, nil
}

// SaveToFile saves config to a JSON file (atomic write). Creates directories as needed.
//
// NOTE: This may include API keys. The file is written with 0600 permissions.
func SaveToFile(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config JSON: %w", err)
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		// Best-effort; don't fail after successful rename.
		_ = err
	}

	return nil
}

// DeleteConfig removes the config file at the given path.
func DeleteConfig(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone, not an error
		}
		return fmt.Errorf("remove config: %w", err)
	}
	return nil
}
