package cache

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultCacheDir returns the platform-appropriate cache directory for commit-coach.
//
// - Linux: ~/.cache/commit-coach/
// - macOS: ~/Library/Caches/commit-coach/
// - Windows: %LocalAppData%\commit-coach\cache\
func DefaultCacheDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Caches")
	case "windows":
		baseDir = os.Getenv("LocalAppData")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(baseDir, "commit-coach", "cache"), nil
	default: // Linux and others
		// Try XDG_CACHE_HOME first
		baseDir = os.Getenv("XDG_CACHE_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".cache")
		}
	}

	return filepath.Join(baseDir, "commit-coach"), nil
}
