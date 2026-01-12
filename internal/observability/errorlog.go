package observability

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/chuckie/commit-coach/internal/security"
)

var (
	initOnce sync.Once
	logFile  *os.File
	logPath  string
	logger   *log.Logger
	redactor = security.NewRedactor()
	initErr  error
)

// Init configures logging to a local error log file.
//
// Default path is ./commit-coach-error.log, override with COMMIT_COACH_LOG_PATH.
// The log is redacted to avoid leaking secrets.
func Init() (path string, cleanup func(), err error) {
	initOnce.Do(func() {
		logPath = os.Getenv("COMMIT_COACH_LOG_PATH")
		if logPath == "" {
			logPath = "commit-coach-error.log"
		}

		dir := filepath.Dir(logPath)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}

		logFile, initErr = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if initErr != nil {
			return
		}

		logger = log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	})

	cleanup = func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}

	return logPath, cleanup, initErr
}

// Logger returns the configured file logger if available.
func Logger() *log.Logger {
	if logger != nil {
		return logger
	}
	return log.Default()
}

// Path returns the configured log file path (empty if Init hasn't run yet).
func Path() string {
	return logPath
}

// RedactForLog removes common secret patterns from logs.
func RedactForLog(s string) string {
	return redactor.RedactLog(s)
}

// Snip returns a safe prefix of s, capped by rune count.
func Snip(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}

	n := 0
	idx := 0
	for idx < len(s) {
		if n >= maxRunes {
			break
		}
		_, size := utf8.DecodeRuneInString(s[idx:])
		if size <= 0 {
			break
		}
		idx += size
		n++
	}

	if idx >= len(s) {
		return s
	}
	return s[:idx] + "…"
}
