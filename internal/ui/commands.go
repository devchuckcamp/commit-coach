package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devchuckcamp/commit-coach/internal/config"
	"github.com/devchuckcamp/commit-coach/internal/domain"
)

// cmdLoadSuggestions loads suggestions asynchronously.
func (m *Model) cmdLoadSuggestions() tea.Msg {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	suggestions, err := m.app.Suggest.SuggestCommits(ctx, m.provider, m.model, m.temperature)
	return msgSuggestionsLoaded{
		suggestions: suggestions,
		err:         err,
	}
}

// cmdCommit commits the selected message.
func (m *Model) cmdCommit() tea.Msg {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.suggestions) {
		return msgCommitComplete{
			hash: "",
			err:  nil,
		}
	}

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	msg := m.suggestions[m.selectedIndex].Format()
	hash, err := m.app.Commit.Commit(ctx, msg, m.dryRun)
	return msgCommitComplete{
		hash: hash,
		err:  err,
	}
}

// handleListKeys handles keybindings in list state.
func (m *Model) handleListKeys(msg tea.KeyMsg) (*Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		if m.selectedIndex < len(m.suggestions)-1 {
			m.selectedIndex++
		}
	case "e":
		m.isEditing = true
		m.state = StateEdit
		if m.selectedIndex < len(m.suggestions) {
			m.editText = m.suggestions[m.selectedIndex].Format()
		}
	case "r":
		m.state = StateLoading
		return m, m.cmdLoadSuggestions
	case "s":
		m.state = StateSetup
		m.setup = NewSetupEmbedded(&config.Config{Provider: m.provider, Model: m.model, OllamaURL: m.ollamaURL})
		return m, nil
	case "n":
		m.dryRun = true
		m.state = StateDryRun
	case "enter":
		m.dryRun = false
		m.state = StateLoading
		return m, m.cmdCommit
	}

	return m, nil
}

// handleEditKeys handles keybindings in edit state.
func (m *Model) handleEditKeys(msg tea.KeyMsg) (*Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		// Save and parse edited message
		// For now, parse simple "type: subject" format
		m.state = StateList
		m.isEditing = false

		// Try to parse the edited text as a new suggestion
		if m.selectedIndex < len(m.suggestions) {
			parsed := m.parseEditedMessage(m.editText)
			if parsed != nil {
				m.suggestions[m.selectedIndex] = *parsed
			}
		}

	case "esc":
		m.state = StateList
		m.isEditing = false
		m.editText = ""
	}

	return m, nil
}

// parseEditedMessage attempts to parse edited message back into suggestion.
// Parses conventional commit format: "type: subject\n\nbody\n\nfooter"
func (m *Model) parseEditedMessage(text string) *domain.Suggestion {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Split into paragraphs (separated by blank lines)
	paragraphs := strings.Split(text, "\n\n")

	// First paragraph is "type: subject"
	firstLine := strings.TrimSpace(paragraphs[0])
	if firstLine == "" {
		return nil
	}

	// Parse type and subject from first line
	var commitType, subject string
	if colonIdx := strings.Index(firstLine, ":"); colonIdx > 0 {
		commitType = strings.TrimSpace(firstLine[:colonIdx])
		subject = strings.TrimSpace(firstLine[colonIdx+1:])
	} else {
		// No colon found, treat entire line as subject with default type
		commitType = "fix"
		subject = firstLine
	}

	// Validate type against known types, fallback to "fix" if invalid
	validType := false
	for _, t := range domain.ValidCommitTypes {
		if commitType == t {
			validType = true
			break
		}
	}
	if !validType {
		// Invalid type: prepend it to subject and use "fix"
		subject = commitType + ": " + subject
		commitType = "fix"
	}

	// Extract body and footer from remaining paragraphs
	var body, footer string
	for i := 1; i < len(paragraphs); i++ {
		para := strings.TrimSpace(paragraphs[i])
		if para == "" {
			continue
		}
		// Check if this paragraph is a footer (BREAKING CHANGE:, Closes:, Refs:)
		if strings.HasPrefix(para, "BREAKING CHANGE:") ||
			strings.HasPrefix(para, "Closes:") ||
			strings.HasPrefix(para, "Refs:") {
			footer = para
		} else if body == "" {
			body = para
		} else {
			// Append additional body paragraphs
			body = body + "\n\n" + para
		}
	}

	return &domain.Suggestion{
		Type:    commitType,
		Subject: subject,
		Body:    body,
		Footer:  footer,
	}
}
