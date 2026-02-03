package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// cmdAnalyzeFiles analyzes staged files asynchronously.
func (m *Model) cmdAnalyzeFiles() tea.Msg {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	result, files, err := m.app.Analyze.AnalyzeFiles(ctx, m.model, m.temperature)
	return msgFileAnalysisComplete{
		result: result,
		files:  files,
		err:    err,
	}
}

// cmdUnstageFiles unstages the unselected files.
func (m *Model) cmdUnstageFiles() tea.Msg {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Find files to unstage (those not selected)
	var toUnstage []string
	for _, f := range m.stagedFiles {
		if !m.selectedFiles[f.Path] {
			toUnstage = append(toUnstage, f.Path)
		}
	}

	if len(toUnstage) == 0 {
		return msgUnstageComplete{err: nil}
	}

	err := m.app.Unstage.Unstage(ctx, toUnstage)
	return msgUnstageComplete{err: err}
}

// handleFileReviewKeys handles keybindings in file review state.
func (m *Model) handleFileReviewKeys(msg tea.KeyMsg) (*Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.fileListCursor > 0 {
			m.fileListCursor--
		}

	case "down", "j":
		if m.fileListCursor < len(m.stagedFiles)-1 {
			m.fileListCursor++
		}

	case " ": // Space toggles selection
		if m.fileListCursor < len(m.stagedFiles) {
			path := m.stagedFiles[m.fileListCursor].Path
			m.selectedFiles[path] = !m.selectedFiles[path]
		}

	case "a": // Select all
		for _, f := range m.stagedFiles {
			m.selectedFiles[f.Path] = true
		}

	case "n": // Select none
		for _, f := range m.stagedFiles {
			m.selectedFiles[f.Path] = false
		}

	case "enter": // Continue with selected files
		// Count selected files
		selectedCount := 0
		for _, selected := range m.selectedFiles {
			if selected {
				selectedCount++
			}
		}

		if selectedCount == 0 {
			// No files selected, show error
			m.state = StateError
			m.err = fmt.Errorf("no files selected for commit")
			return m, nil
		}

		// If all files selected, skip unstaging
		if selectedCount == len(m.stagedFiles) {
			m.state = StateLoading
			return m, m.cmdLoadSuggestions
		}

		// Unstage unselected files
		m.state = StateLoading
		return m, m.cmdUnstageFiles

	case "esc": // Skip review, use all files
		m.state = StateLoading
		return m, m.cmdLoadSuggestions
	}

	return m, nil
}

// viewFileReview renders the file review state.
func (m *Model) viewFileReview() string {
	var sb strings.Builder

	sb.WriteString("Warning: Staged files may be unrelated\n")
	sb.WriteString("=========================================\n\n")

	if m.analysisResult != nil && m.analysisResult.Reasoning != "" {
		sb.WriteString(m.analysisResult.Reasoning)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Select files to include in this commit:\n\n")

	// Render file list with checkboxes
	for i, f := range m.stagedFiles {
		cursor := "  "
		if i == m.fileListCursor {
			cursor = "> "
		}

		checkbox := "[ ]"
		if m.selectedFiles[f.Path] {
			checkbox = "[x]"
		}

		statusLabel := statusToLabel(f.Status)
		sb.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, checkbox, statusLabel, f.Path))
	}

	// Show groups if available
	if len(m.fileGroups) > 1 {
		sb.WriteString("\nSuggested groups:\n")
		for i, g := range m.fileGroups {
			sb.WriteString(fmt.Sprintf("  %d. %s (%.0f%% confidence)\n", i+1, g.Label, g.Confidence*100))
			for _, path := range g.Files {
				sb.WriteString(fmt.Sprintf("     - %s\n", path))
			}
		}
	}

	sb.WriteString("\nKeybindings:\n")
	sb.WriteString("  Up/Down  Navigate\n")
	sb.WriteString("  Space    Toggle selection\n")
	sb.WriteString("  a        Select all\n")
	sb.WriteString("  n        Select none\n")
	sb.WriteString("  Enter    Continue with selected files\n")
	sb.WriteString("  Esc      Skip review, use all files\n")
	sb.WriteString("  Ctrl+C   Exit\n")

	return sb.String()
}

// statusToLabel converts a git status code to a human-readable label.
func statusToLabel(status string) string {
	switch status {
	case "A":
		return "added   "
	case "M":
		return "modified"
	case "D":
		return "deleted "
	case "R":
		return "renamed "
	default:
		return status + "       "
	}
}
