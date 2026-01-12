package llm

import (
	"fmt"

	"github.com/chuckie/commit-coach/internal/adapters/llm/groq"
	"github.com/chuckie/commit-coach/internal/adapters/llm/mock"
	"github.com/chuckie/commit-coach/internal/adapters/llm/ollama"
	"github.com/chuckie/commit-coach/internal/adapters/llm/openai"
	"github.com/chuckie/commit-coach/internal/ports"
)

// NewFromConfig creates a new LLM provider from configuration.
func NewFromConfig(provider, apiKey, baseURL, ollamaURL, model string) (ports.LLM, error) {
	switch provider {
	case "openai":
		return openai.NewClient(apiKey, baseURL)
	case "groq":
		return groq.NewClient(apiKey, model), nil
	case "ollama":
		return ollama.NewClient(ollamaURL, model), nil
	case "mock":
		return mock.NewClient(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

