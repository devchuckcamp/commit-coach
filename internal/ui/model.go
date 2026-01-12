package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chuckie/commit-coach/internal/app"
	"github.com/chuckie/commit-coach/internal/config"
	"github.com/chuckie/commit-coach/internal/domain"
	"github.com/chuckie/commit-coach/internal/ports"
)

// Model is the main Bubble Tea model.
type Model struct {
	app           *app.App
	state         State
	setup         *SetupModel
	suggestions   []domain.Suggestion
	selectedIndex int
	editText      string
	isEditing     bool
	dryRun        bool
	provider      string
	model         string
	temperature   float32
	baseURL       string
	ollamaURL     string
	llmFactory    func(provider, apiKey, baseURL, ollamaURL, model string) (ports.LLM, error)
	spinner       spinner.Model
	width         int
	height        int
	err           error
	lastHash      string
}

// State represents the current UI state.
type State int

const (
	StateLoading State = iota
	StateSetup
	StateList
	StateEdit
	StateDryRun
	StateSuccess
	StateError
)

// New creates a new UI model.
func New(app *app.App, provider, model string, temperature float32, baseURL, ollamaURL string, llmFactory func(provider, apiKey, baseURL, ollamaURL, model string) (ports.LLM, error)) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Model{
		app:           app,
		state:         StateLoading,
		selectedIndex: 0,
		provider:      provider,
		model:         model,
		temperature:   temperature,
		baseURL:       baseURL,
		ollamaURL:     ollamaURL,
		llmFactory:    llmFactory,
		spinner:       s,
		width:         80,
		height:        24,
		err:           nil,
	}
}

// Init initializes the model and starts the suggestion loading.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.cmdLoadSuggestions)
}

// Update handles messages and state transitions.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state != StateSetup {
				return m, tea.Quit
			}
		}

		// State-specific key handling
		switch m.state {
		case StateLoading:
			// No keys during loading

		case StateSetup:
			if m.setup == nil {
				m.setup = NewSetupEmbedded(&config.Config{Provider: m.provider, Model: m.model, OllamaURL: m.ollamaURL})
			}
			child, cmd := m.setup.Update(msg)
			if sm, ok := child.(*SetupModel); ok {
				m.setup = sm
			}
			return m, cmd

		case StateList:
			m2, cmd := m.handleListKeys(msg)
			if cmd != nil {
				return m2, cmd
			}
			m = m2

		case StateEdit:
			m2, cmd := m.handleEditKeys(msg)
			if cmd != nil {
				return m2, cmd
			}
			m = m2

		case StateDryRun:
			// Any key returns to list
			m.state = StateList

		case StateSuccess:
			// Any key exits
			return m, tea.Quit

		case StateError:
			// Any key returns to list
			m.state = StateList
			m.err = nil
		}

	case msgSuggestionsLoaded:
		if msg.err != nil {
			m.state = StateError
			m.err = msg.err
		} else {
			m.suggestions = msg.suggestions
			m.selectedIndex = 0
			m.state = StateList
		}

	case msgCommitComplete:
		if msg.err != nil {
			m.state = StateError
			m.err = msg.err
		} else {
			m.state = StateSuccess
			m.lastHash = msg.hash
			// Give the user a moment to see the success message, then exit.
			return m, tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
				return msgAutoQuit{}
			})
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case msgSetupFinished:
		m.setup = nil
		if !msg.confirmed {
			m.state = StateList
			return m, nil
		}

		m.provider = msg.provider
		m.model = msg.model

		apiKey := msg.apiKey
		if m.llmFactory == nil {
			m.state = StateError
			m.err = fmt.Errorf("LLM factory not configured")
			return m, nil
		}

		// Best-effort persistence so setup changes are remembered across runs.
		if path, err := config.DefaultConfigPath(); err == nil {
			persisted, _ := config.Load() // may be partially invalid; best-effort
			if persisted == nil {
				persisted = &config.Config{}
			}
			persisted.Provider = m.provider
			persisted.Model = m.model
			switch m.provider {
			case "openai", "groq", "anthropic":
				persisted.APIKey = apiKey
			case "ollama":
				persisted.APIKey = "ollama"
			case "mock":
				persisted.APIKey = "mock"
			}
			_ = config.SaveToFile(path, persisted) // ignore persistence errors in UI flow
		}

		llm, err := m.llmFactory(m.provider, apiKey, m.baseURL, m.ollamaURL, m.model)
		if err != nil {
			m.state = StateError
			m.err = err
			return m, nil
		}
		m.app.Suggest.SetLLM(llm)
		m.state = StateLoading
		return m, m.cmdLoadSuggestions

	case msgAutoQuit:
		if m.state == StateSuccess {
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the current state.
func (m *Model) View() string {
	switch m.state {
	case StateLoading:
		return m.viewLoading()
	case StateSetup:
		if m.setup == nil {
			m.setup = NewSetupEmbedded(&config.Config{Provider: m.provider, Model: m.model, OllamaURL: m.ollamaURL})
		}
		return m.setup.View()
	case StateList:
		return m.viewList()
	case StateEdit:
		return m.viewEdit()
	case StateDryRun:
		return m.viewDryRun()
	case StateSuccess:
		return m.viewSuccess()
	case StateError:
		return m.viewError()
	default:
		return ""
	}
}

// viewLoading renders the loading state.
func (m *Model) viewLoading() string {
	return m.spinner.View() + " Generating suggestions..."
}

// viewList renders the suggestion list.
func (m *Model) viewList() string {
	if len(m.suggestions) == 0 {
		return "No suggestions available."
	}

	var output string
	output += "Suggestions:\n\n"

	for i, s := range m.suggestions {
		prefix := "  "
		if i == m.selectedIndex {
			prefix = "> "
		}
		output += prefix + s.Format() + "\n\n"
	}

	output += "\nKeybindings:\n"
	output += "  ↑/↓    Navigate\n"
	output += "  e      Edit\n"
	output += "  r      Regenerate\n"
	output += "  s      Setup (switch provider/model)\n"
	output += "  n      Dry-run\n"
	output += "  Enter  Commit\n"
	output += "  Ctrl+C Exit\n"

	return output
}

// viewEdit renders the edit state.
func (m *Model) viewEdit() string {
	return "Edit message:\n\n" + m.editText + "\n\n(Ctrl+S to save, Esc to cancel)"
}

// viewDryRun renders the dry-run preview.
func (m *Model) viewDryRun() string {
	return "Dry-run preview:\n\ngit commit -m \"" + m.suggestions[m.selectedIndex].Format() + "\"\n\n(Press any key to continue)"
}

// viewSuccess renders the success state.
func (m *Model) viewSuccess() string {
	return "✓ Committed as " + m.lastHash + "\nExiting...\n"
}

// viewError renders the error state.
func (m *Model) viewError() string {
	return "Error: " + m.err.Error() + "\n\n(Press any key to return)"
}

// Custom messages
type msgSuggestionsLoaded struct {
	suggestions []domain.Suggestion
	err         error
}

type msgCommitComplete struct {
	hash string
	err  error
}

type msgSetupFinished struct {
	provider  string
	model     string
	apiKey    string
	confirmed bool
}

type msgAutoQuit struct{}
