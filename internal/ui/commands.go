package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chuckie/commit-coach/internal/config"
	"github.com/chuckie/commit-coach/internal/domain"
)

// cmdLoadSuggestions loads suggestions asynchronously.
func (m *Model) cmdLoadSuggestions() tea.Msg {
	ctx := context.Background()
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

	ctx := context.Background()
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
func (m *Model) parseEditedMessage(text string) *domain.Suggestion {
	// Simple parsing: "type: subject" or multiline with body
	// For MVP, just update the subject if it's simple format
	lines := len(text) > 0
	if !lines {
		return nil
	}

	// Return a minimal suggestion for now
	// TODO: improve parsing
	return &domain.Suggestion{
		Type:    "fix",
		Subject: text,
		Body:    "",
		Footer:  "",
	}
}
