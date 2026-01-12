package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chuckie/commit-coach/internal/adapters/cache"
	"github.com/chuckie/commit-coach/internal/adapters/git"
	"github.com/chuckie/commit-coach/internal/adapters/llm"
	"github.com/chuckie/commit-coach/internal/app"
	"github.com/chuckie/commit-coach/internal/config"
	"github.com/chuckie/commit-coach/internal/observability"
	"github.com/chuckie/commit-coach/internal/ui"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	// Best-effort error logging to a local file.
	if _, cleanup, err := observability.Init(); err == nil {
		defer cleanup()
	}

	if len(args) >= 2 {
		switch args[1] {
		case "-h", "--help", "help":
			printHelp()
			return 0
		case "setup":
			return runSetup(args[2:])
		case "config":
			return runConfig(args[2:])
		case "suggest":
			return runSuggest(args[2:])
		default:
			if strings.HasPrefix(args[1], "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n\n", args[1])
			} else {
				fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[1])
			}
			printHelp()
			return 2
		}
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Fallback: even if the sentinel wrapper is lost, a missing key for
		// openai/groq/anthropic should always trigger interactive setup.
		needsSetup := config.IsSetupRequired(err) || (cfg != nil && (cfg.Provider == "openai" || cfg.Provider == "groq" || cfg.Provider == "anthropic") && cfg.APIKey == "")
		if needsSetup {
			setup := ui.NewSetup(cfg)
			p := tea.NewProgram(setup)
			finalModel, runErr := p.Run()
			if runErr != nil {
				fmt.Fprintf(os.Stderr, "Setup error: %v\n", runErr)
				return 1
			}

			sm, ok := finalModel.(*ui.SetupModel)
			if !ok {
				fmt.Fprintf(os.Stderr, "Setup error: unexpected model type\n")
				return 1
			}

			provider, model, apiKey, confirmed := sm.Result()
			if !confirmed {
				fmt.Fprintf(os.Stderr, "Setup cancelled.\n")
				return 1
			}

			// Apply to runtime config for this run.
			cfg.Provider = provider
			cfg.Model = model
			switch provider {
			case "openai", "groq", "anthropic":
				cfg.APIKey = apiKey
			case "ollama":
				cfg.APIKey = "ollama"
			case "mock":
				cfg.APIKey = "mock"
			}

			if path, err := config.DefaultConfigPath(); err == nil {
				if err := config.SaveToFile(path, cfg); err == nil {
					fmt.Fprintf(os.Stderr, "Saved config to %s\n", path)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			return 1
		}
	}

	// Create adapters
	gitAdapter := git.NewExecutor()
	cacheAdapter := cache.NewInMemory()

	// Use factory to create LLM provider
	llmAdapter, err := llm.NewFromConfig(cfg.Provider, cfg.APIKey, cfg.BaseURL, cfg.OllamaURL, cfg.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM provider: %v\n", err)
		return 1
	}

	// Create application
	application := app.NewApp(llmAdapter, gitAdapter, cacheAdapter, cfg.DiffCap, cfg.UseCache)

	// Create TUI model
	model := ui.New(application, cfg.Provider, cfg.Model, cfg.Temperature, cfg.BaseURL, cfg.OllamaURL, llm.NewFromConfig)

	// Run TUI
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
	return 0
}

func printHelp() {
	fmt.Fprintln(os.Stdout, "commit-coach — AI-powered commit message suggestions")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  commit-coach            # Launch TUI")
	fmt.Fprintln(os.Stdout, "  commit-coach setup      # Setup (persisted; interactive by default)")
	fmt.Fprintln(os.Stdout, "  commit-coach config     # Show config path + active config")
	fmt.Fprintln(os.Stdout, "  commit-coach suggest    # Print 3 suggestions (non-TUI)")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Commands:")
	fmt.Fprintln(os.Stdout, "  setup [--provider P] [--model M] [--api-key K]")
	fmt.Fprintln(os.Stdout, "  config [path|set --provider P --model M [--api-key K]]")
	fmt.Fprintln(os.Stdout, "  suggest [--json]")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Common flags:")
	fmt.Fprintln(os.Stdout, "  -h, --help              Show help")
}

func runSetup(args []string) int {
	// Minimal flags (no external deps):
	// setup [--provider <p>] [--model <m>] [--api-key <k>]
	var provider, model, apiKey string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "Usage: commit-coach setup [--provider P] [--model M] [--api-key K]")
			return 0
		case "--provider":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--provider requires a value")
				return 2
			}
			provider = args[i]
		case "--model":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--model requires a value")
				return 2
			}
			model = args[i]
		case "--api-key":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "--api-key requires a value")
				return 2
			}
			apiKey = args[i]
		default:
			fmt.Fprintf(os.Stderr, "Unknown setup flag/arg: %s\n", args[i])
			return 2
		}
	}

	cfg, _ := config.Load() // best-effort; if it errors, wizard can still run.
	if cfg == nil {
		cfg = &config.Config{}
	}

	if provider != "" {
		cfg.Provider = provider
	}
	if model != "" {
		cfg.Model = model
	}
	if apiKey != "" {
		cfg.APIKey = apiKey
	}

	if cfg.Provider != "" && cfg.Model != "" && (cfg.Provider == "mock" || cfg.Provider == "ollama" || cfg.APIKey != "") {
		if cfg.Provider == "mock" {
			cfg.APIKey = "mock"
		}
		if cfg.Provider == "ollama" {
			cfg.APIKey = "ollama"
		}
		path, err := config.DefaultConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to determine config path: %v\n", err)
			return 1
		}
		if err := config.SaveToFile(path, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Saved config to %s\n", path)
		return 0
	}

	setup := ui.NewSetup(cfg)
	p := tea.NewProgram(setup)
	finalModel, runErr := p.Run()
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Setup error: %v\n", runErr)
		return 1
	}

	sm, ok := finalModel.(*ui.SetupModel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Setup error: unexpected model type\n")
		return 1
	}

	provider, model, apiKey, confirmed := sm.Result()
	if !confirmed {
		fmt.Fprintf(os.Stderr, "Setup cancelled.\n")
		return 1
	}

	cfg.Provider = provider
	cfg.Model = model
	switch provider {
	case "openai", "groq", "anthropic":
		cfg.APIKey = apiKey
	case "ollama":
		cfg.APIKey = "ollama"
	case "mock":
		cfg.APIKey = "mock"
	}

	path, err := config.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine config path: %v\n", err)
		return 1
	}
	if err := config.SaveToFile(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Saved config to %s\n", path)
	return 0
}

func runConfig(args []string) int {
	path, err := config.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine config path: %v\n", err)
		return 1
	}

	if len(args) >= 1 {
		switch args[0] {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "Usage:")
			fmt.Fprintln(os.Stdout, "  commit-coach config")
			fmt.Fprintln(os.Stdout, "  commit-coach config path")
			fmt.Fprintln(os.Stdout, "  commit-coach config set --provider P --model M [--api-key K]")
			return 0
		case "path":
			fmt.Fprintln(os.Stdout, path)
			return 0
		case "set":
			var provider, model, apiKey string
			for i := 1; i < len(args); i++ {
				switch args[i] {
				case "--provider":
					i++
					if i >= len(args) {
						fmt.Fprintln(os.Stderr, "--provider requires a value")
						return 2
					}
					provider = args[i]
				case "--model":
					i++
					if i >= len(args) {
						fmt.Fprintln(os.Stderr, "--model requires a value")
						return 2
					}
					model = args[i]
				case "--api-key":
					i++
					if i >= len(args) {
						fmt.Fprintln(os.Stderr, "--api-key requires a value")
						return 2
					}
					apiKey = args[i]
				default:
					fmt.Fprintf(os.Stderr, "Unknown config set flag/arg: %s\n", args[i])
					return 2
				}
			}

			cfg, _ := config.Load() // best-effort; may fail when setup required
			if cfg == nil {
				cfg = &config.Config{}
			}
			if provider != "" {
				cfg.Provider = provider
			}
			if model != "" {
				cfg.Model = model
			}
			if apiKey != "" {
				cfg.APIKey = apiKey
			}

			switch cfg.Provider {
			case "mock":
				cfg.APIKey = "mock"
			case "ollama":
				cfg.APIKey = "ollama"
			case "openai", "groq", "anthropic":
				if strings.TrimSpace(cfg.APIKey) == "" {
					fmt.Fprintf(os.Stderr, "API key is required for provider %s (pass --api-key or set env var)\n", cfg.Provider)
					return 2
				}
			default:
				fmt.Fprintf(os.Stderr, "Invalid provider: %s\n", cfg.Provider)
				return 2
			}

			if err := config.SaveToFile(path, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
				return 1
			}
			fmt.Fprintf(os.Stdout, "Saved config to %s\n", path)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
			return 2
		}
	}

	cfg, err := config.Load()
	if cfg == nil {
		cfg = &config.Config{}
	}
	if err != nil && !config.IsSetupRequired(err) {
		fmt.Fprintf(os.Stderr, "Config warning: %v\n", err)
	}

	keyStatus := "(missing)"
	if cfg.Provider == "mock" || cfg.Provider == "ollama" {
		keyStatus = "(not required)"
	} else if cfg.APIKey != "" {
		keyStatus = "(set)"
	}

	fmt.Fprintf(os.Stdout, "Config path: %s\n", path)
	fmt.Fprintf(os.Stdout, "Provider:    %s\n", cfg.Provider)
	fmt.Fprintf(os.Stdout, "Model:       %s\n", cfg.Model)
	fmt.Fprintf(os.Stdout, "API key:     %s\n", keyStatus)
	return 0
}

func runSuggest(args []string) int {
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "Usage: commit-coach suggest [--json]")
			return 0
		case "--json":
			jsonOut = true
		default:
			fmt.Fprintf(os.Stderr, "Unknown suggest flag/arg: %s\n", args[i])
			return 2
		}
	}

	cfg, err := config.Load()
	if err != nil {
		if config.IsSetupRequired(err) {
			fmt.Fprintln(os.Stderr, "Setup required. Run: commit-coach setup")
			return 1
		}
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		return 1
	}

	gitAdapter := git.NewExecutor()
	cacheAdapter := cache.NewInMemory()
	llmAdapter, err := llm.NewFromConfig(cfg.Provider, cfg.APIKey, cfg.BaseURL, cfg.OllamaURL, cfg.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM provider: %v\n", err)
		return 1
	}
	application := app.NewApp(llmAdapter, gitAdapter, cacheAdapter, cfg.DiffCap, cfg.UseCache)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suggestions, err := application.Suggest.SuggestCommits(ctx, cfg.Provider, cfg.Model, cfg.Temperature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if jsonOut {
		b, err := json.MarshalIndent(suggestions, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(os.Stdout, string(b))
		return 0
	}

	for i, s := range suggestions {
		fmt.Fprintf(os.Stdout, "%d) %s: %s\n", i+1, s.Type, s.Subject)
		if strings.TrimSpace(s.Body) != "" {
			fmt.Fprintf(os.Stdout, "\n%s\n", strings.TrimSpace(s.Body))
		}
		if strings.TrimSpace(s.Footer) != "" {
			fmt.Fprintf(os.Stdout, "\n%s\n", strings.TrimSpace(s.Footer))
		}
		fmt.Fprintln(os.Stdout, "")
	}
	return 0
}
