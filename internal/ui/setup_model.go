package ui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/chuckie/commit-coach/internal/config"
)

type setupStep int

type setupMode int

const (
	setupStepProvider setupStep = iota
	setupStepModel
	setupStepAPIKey
	setupStepConfirm
	setupStepDone
)

const (
	setupModeStandalone setupMode = iota
	setupModeEmbedded
)

// SetupModel is an interactive setup wizard.
// It does not call any LLMs or write files.
type SetupModel struct {
	mode setupMode
	step setupStep

	providers     []string
	providerIndex int
	provider      string
	models        []string
	modelIndex    int
	model         string
	apiKeyInput   textinput.Model
	ollamaURL     string

	completed bool

	err error
}

func NewSetup(cfg *config.Config) *SetupModel {
	providers := []string{"openai", "anthropic", "groq", "ollama", "mock"}

	keyIn := textinput.New()
	keyIn.Prompt = "API key: "
	keyIn.EchoMode = textinput.EchoPassword
	keyIn.EchoCharacter = '*'
	keyIn.CharLimit = 200

	provider := "openai"
	ollamaURL := "http://localhost:11434"
	if cfg != nil {
		if cfg.Provider != "" {
			provider = cfg.Provider
		}
		if cfg.OllamaURL != "" {
			ollamaURL = cfg.OllamaURL
		}
	}

	// Align selection index with provider
	providerIndex := 0
	for i, p := range providers {
		if p == provider {
			providerIndex = i
			break
		}
	}

	models := config.ProviderModels[provider]
	if len(models) == 0 {
		models = []string{""}
	}
	modelIndex := 0
	model := models[0]
	if cfg != nil && cfg.Model != "" {
		for i, m := range models {
			if m == cfg.Model {
				modelIndex = i
				model = m
				break
			}
		}
	}

	return &SetupModel{
		mode:          setupModeStandalone,
		step:          setupStepProvider,
		providers:     providers,
		providerIndex: providerIndex,
		provider:      provider,
		models:        models,
		modelIndex:    modelIndex,
		model:         model,
		apiKeyInput:   keyIn,
		ollamaURL:     ollamaURL,
	}
}

func NewSetupEmbedded(cfg *config.Config) *SetupModel {
	m := NewSetup(cfg)
	m.mode = setupModeEmbedded
	return m
}

func (m *SetupModel) Init() tea.Cmd {
	return nil
}

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear any previous validation error on input/navigation.
		// Errors should not lock the user out of the wizard.
		if m.err != nil {
			m.err = nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.mode == setupModeEmbedded {
				m.completed = false
				m.step = setupStepDone
				return m, func() tea.Msg {
					return msgSetupFinished{confirmed: false}
				}
			}
			return m, tea.Quit
		}

		switch m.step {
		case setupStepProvider:
			return m.updateProvider(msg)
		case setupStepModel:
			return m.updateModel(msg)
		case setupStepAPIKey:
			return m.updateTextStep(msg, &m.apiKeyInput, func() {
				m.provider = m.providers[m.providerIndex]
				m.step = setupStepConfirm
			})
		case setupStepConfirm:
			return m.updateConfirm(msg)
		case setupStepDone:
			if m.mode == setupModeEmbedded {
				return m, nil
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *SetupModel) View() string {
	var v string
	switch m.step {
	case setupStepProvider:
		v = m.viewProvider()
	case setupStepModel:
		v = m.viewModel()
	case setupStepAPIKey:
		v = m.viewText("API key", "Enter your provider API key. Paste with Ctrl+V (or your terminal paste).", m.apiKeyInput.View())
	case setupStepConfirm:
		v = m.viewConfirm()
	case setupStepDone:
		v = "Setup complete.\n"
	default:
		v = ""
	}

	if m.err != nil {
		v += "\nError: " + m.err.Error() + "\n"
	}
	return v
}

func (m *SetupModel) updateProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerIndex > 0 {
			m.providerIndex--
		}
	case "down", "j":
		if m.providerIndex < len(m.providers)-1 {
			m.providerIndex++
		}
	case "enter":
		m.provider = m.providers[m.providerIndex]
		m.models = config.ProviderModels[m.provider]
		if len(m.models) == 0 {
			m.models = []string{""}
		}
		m.modelIndex = 0
		m.model = m.models[0]
		m.step = setupStepModel
	}

	return m, nil
}

func (m *SetupModel) updateModel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.modelIndex > 0 {
			m.modelIndex--
			m.model = m.models[m.modelIndex]
		}
	case "down", "j":
		if m.modelIndex < len(m.models)-1 {
			m.modelIndex++
			m.model = m.models[m.modelIndex]
		}
	case "esc":
		m.step = setupStepProvider
	case "enter":
		m.provider = m.providers[m.providerIndex]
		m.model = m.models[m.modelIndex]
		m.step = nextStepAfterModel(m.provider)
		if m.step == setupStepAPIKey {
			m.apiKeyInput.Focus()
			m.apiKeyInput.CursorEnd()
		}
	}
	return m, nil
}

func (m *SetupModel) updateTextStep(msg tea.KeyMsg, input *textinput.Model, onEnter func()) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.step = setupStepProvider
		input.Blur()
		return m, nil
	case "ctrl+v", "ctrl+shift+v", "shift+insert":
		clip, err := clipboard.ReadAll()
		if err != nil {
			m.err = fmt.Errorf("clipboard paste failed: %w", err)
			return m, nil
		}
		clip = strings.ReplaceAll(clip, "\r", "")
		clip = strings.ReplaceAll(clip, "\n", "")
		if strings.TrimSpace(clip) == "" {
			m.err = fmt.Errorf("clipboard is empty")
			return m, nil
		}
		input.SetValue(input.Value() + clip)
		input.CursorEnd()
		return m, nil
	case "enter":
		val := strings.TrimSpace(input.Value())
		if val == "" {
			m.err = fmt.Errorf("value cannot be empty")
			return m, nil
		}
		onEnter()
		input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	return m, cmd
}

func (m *SetupModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		cfg, err := m.buildRuntimeConfig()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.completed = true
		m.step = setupStepDone
		if m.mode == setupModeEmbedded {
			return m, func() tea.Msg {
				return msgSetupFinished{
					provider:  cfg.Provider,
					model:     cfg.Model,
					apiKey:    cfg.APIKey,
					confirmed: true,
				}
			}
		}
		return m, tea.Quit
	case "n":
		m.step = setupStepProvider
		return m, nil
	}

	return m, nil
}

func (m *SetupModel) viewProvider() string {
	var b strings.Builder
	b.WriteString("commit-coach setup\n\n")
	b.WriteString("Select an AI provider:\n\n")
	for i, p := range m.providers {
		prefix := "  "
		if i == m.providerIndex {
			prefix = "> "
		}
		b.WriteString(prefix + p + "\n")
	}
	b.WriteString("\nKeys: ↑/↓ select, Enter next, q quit\n")
	return b.String()
}

func (m *SetupModel) viewModel() string {
	var b strings.Builder
	b.WriteString("commit-coach setup\n\n")
	b.WriteString("Select a model:\n\n")
	for i, model := range m.models {
		prefix := "  "
		if i == m.modelIndex {
			prefix = "> "
		}
		b.WriteString(prefix + model + "\n")
	}
	b.WriteString("\nKeys: ↑/↓ select, Enter next, Esc back, q quit\n")
	return b.String()
}

func (m *SetupModel) viewText(title, hint, inputView string) string {
	return fmt.Sprintf(
		"commit-coach setup\n\n%s\n%s\n\n%s\n\nKeys: Enter next, Esc back, q quit\n",
		title,
		hint,
		inputView,
	)
}

func (m *SetupModel) viewConfirm() string {
	provider := m.providers[m.providerIndex]
	model := m.model
	apiKey := strings.TrimSpace(m.apiKeyInput.Value())

	apiKeyStatus := "(not required)"
	if provider == "openai" || provider == "groq" || provider == "anthropic" {
		apiKeyStatus = maskSecret(apiKey)
	}

	lines := []string{
		"commit-coach setup\n",
		"Start commit-coach with:\n",
		fmt.Sprintf("Provider:   %s", provider),
		fmt.Sprintf("Model:      %s", model),
		fmt.Sprintf("API key:    %s", apiKeyStatus),
	}
	lines = append(lines,
		"\nContinue? (y/n)")

	return strings.Join(lines, "\n") + "\n"
}

func (m *SetupModel) buildRuntimeConfig() (*config.Config, error) {
	provider := m.providers[m.providerIndex]
	model := strings.TrimSpace(m.model)
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	switch provider {
	case "openai":
		key := strings.TrimSpace(m.apiKeyInput.Value())
		if key == "" {
			return nil, fmt.Errorf("API key is required for openai")
		}
		return &config.Config{Provider: provider, Model: model, APIKey: key}, nil
	case "anthropic":
		key := strings.TrimSpace(m.apiKeyInput.Value())
		if key == "" {
			return nil, fmt.Errorf("API key is required for anthropic")
		}
		return &config.Config{Provider: provider, Model: model, APIKey: key}, nil
	case "groq":
		key := strings.TrimSpace(m.apiKeyInput.Value())
		if key == "" {
			return nil, fmt.Errorf("API key is required for groq")
		}
		return &config.Config{Provider: provider, Model: model, APIKey: key}, nil
	case "ollama":
		return &config.Config{Provider: provider, Model: model, APIKey: "ollama", OllamaURL: m.ollamaURL}, nil
	case "mock":
		return &config.Config{Provider: provider, Model: model, APIKey: "mock"}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", provider)
	}
}

func nextStepAfterModel(provider string) setupStep {
	if provider == "openai" || provider == "groq" || provider == "anthropic" {
		return setupStepAPIKey
	}
	return setupStepConfirm
}

func maskSecret(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(missing)"
	}
	if len(v) <= 6 {
		return "******"
	}
	return v[:3] + strings.Repeat("*", len(v)-6) + v[len(v)-3:]
}

// Result returns the selected provider/model/apikey.
// ok is true only when the user confirmed the setup.
func (m *SetupModel) Result() (provider, model, apiKey string, ok bool) {
	provider = m.providers[m.providerIndex]
	model = m.model
	apiKey = strings.TrimSpace(m.apiKeyInput.Value())
	return provider, model, apiKey, m.completed
}
