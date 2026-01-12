package config

// ProviderModels lists supported model options per provider.
// Used by the interactive installer UI.
var ProviderModels = map[string][]string{
	"anthropic": {
		"claude-3-5-sonnet-latest",
		"claude-3-5-haiku-latest",
		"claude-3-opus-latest",
	},
	"groq": {
		"llama-3.1-8b-instant",
		"llama-3.3-70b-versatile",
		"openai/gpt-oss-20b",
		"openai/gpt-oss-120b",
		"groq/compound",
		"groq/compound-mini",
	},
	"openai": {
		"gpt-5.2",
		"gpt-5-mini",
		"gpt-5-nano",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"gpt-4o",
		"gpt-4o-mini",
	},
	"ollama": {
		"qwen2.5-coder",
		"qwen3-coder",
		"codellama",
		"deepseek-coder",
		"llama3.1",
		"llama3.2",
		"gemma2",
		"mistral",
	},
	"mock": {"mock"},
}
