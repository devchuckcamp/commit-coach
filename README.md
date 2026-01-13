# commit-coach

An interactive terminal app for generating AI-powered Conventional Commit messages.

## Features

-  Generates 3 Conventional Commit suggestions based on staged changes
-  Redacts secrets before sending diffs to LLM providers
-  Provider-agnostic: supports OpenAI and Groq (extensible)
-  Lightweight Bubble Tea TUI with preview and edit support
-  Atomic git commits with dry-run mode
-  Optional caching by diff hash for faster regeneration
-  Comprehensive test coverage with golden tests

## Quick Start

### Prerequisites
- Go 1.21+
- An API key from OpenAI or Groq
- Bash/Zsh shell (or WSL on Windows)

### Installation

```bash
cd commit-coach
go build -o commit-coach ./
```

To install so you can run `commit-coach` from anywhere:

```bash
cd commit-coach
bash ./scripts/install.sh

# then:
commit-coach --help
```

The installer also creates a local build artifact in `dist/` (e.g., `dist/commit-coach.exe` on Windows).

## Setup by OS

### macOS

1. Install Go 1.21+
2. Build + install (user-local, no sudo):

```bash
cd commit-coach
bash ./scripts/install.sh
```

If `~/.local/bin` is not on your PATH, add this to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Verify:

```bash
commit-coach --help
```

Uninstall:

```bash
cd commit-coach
bash ./scripts/uninstall.sh
```

### Linux

1. Install Go 1.21+
2. Build + install (user-local, no sudo):

```bash
cd commit-coach
bash ./scripts/install.sh
```

If `~/.local/bin` is not on your PATH, add this to your shell profile (`~/.zshrc` by default on macOS):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Verify:

```bash
commit-coach --help
```

Uninstall:

```bash
cd commit-coach
bash ./scripts/uninstall.sh
```

### Windows

**Recommended terminal:** Git Bash or WSL.

1. Install Go 1.21+
2. Build + install:

```bash
cd commit-coach
bash ./scripts/install.sh
```

Notes:
- The installer defaults to `/c/Program Files/commit-coach` (Node.js-style). If your terminal isn’t elevated, it falls back to `/c/Users/<you>/.local/bin` and prints a PATH hint.
- If you want Program Files, re-run from an elevated terminal (Run as Administrator).

Verify:

```bash
commit-coach --help
commit-coach config
```

Uninstall:

```bash
cd commit-coach
bash ./scripts/uninstall.sh
```

### Configuration

On first run, `commit-coach` will prompt you to select a provider/model and enter an API key (when required). The selections are persisted to a per-user config file so you only have to do it once.

You can re-run setup anytime:

```bash
./commit-coach setup
```

If you prefer non-interactive configuration, you can set environment variables:

```bash
# Provider + model
export LLM_PROVIDER="openai"          # openai|anthropic|groq|ollama|mock (default: openai)
export LLM_MODEL="gpt-4o-mini"        # default: gpt-4o-mini
export LLM_TEMPERATURE="0.7"          # default: 0.7

# Provider credentials / endpoints
export OPENAI_API_KEY="sk-..."        # required for provider=openai
export ANTHROPIC_API_KEY="..."        # required for provider=anthropic
export GROQ_API_KEY="..."             # required for provider=groq
export OPENAI_BASE_URL=""             # optional (default: empty)
export OLLAMA_URL="http://localhost:11434"  # optional

# App behavior
export DIFF_CAP_BYTES="8192"          # default: 8192
export CONFIRM_BEFORE_SEND="true"     # default: true
export DRY_RUN="false"               # default: false
export REDACT_SECRETS="true"          # default: true
export ENABLE_CACHE="true"            # default: true
```

### Usage

1. Stage your changes:
```bash
git add <files>
```

2. Run commit-coach:
```bash
./commit-coach
```

Other commands:

```bash
./commit-coach --help
./commit-coach config
./commit-coach config path
./commit-coach config set --provider openai --model gpt-4o-mini --api-key sk-...
./commit-coach suggest
./commit-coach suggest --json
```

3. Navigate suggestions with ↑/↓, press Enter to commit:
```
> Navigate suggestions (↑/↓ to select, e to edit, r to regenerate, n for dry-run, Enter to commit)
```

Tip: press `s` in the list view to reopen setup and switch provider/model mid-session.

## Architecture

Follows Clean Architecture with strict layering:

- **UI Layer**: Bubble Tea TUI with single-owner state management
- **Application Layer**: Use-cases (SuggestService, CommitService)
- **Domain Layer**: Commit models, validation rules, formatting
- **Ports Layer**: Interfaces (LLM, Git, Redactor, Cache, Clock)
- **Adapters Layer**: Provider clients, git executor, redaction rules

## Testing

```bash
# Run all tests
make test

# Run with race detector (required for CI)
make test-race

# Run with coverage
make test-coverage

# Run specific tests
go test -v ./internal/domain -run TestSuggestion
```

## Design Decisions

### Atomicity
Commits are all-or-nothing: either the commit succeeds fully, or nothing changes.

### Concurrency
All I/O operations run in Bubble Tea `tea.Cmd` to avoid blocking the TUI. No goroutine leaks.

### Security
- Redacts API keys, auth headers, and credentials before LLM calls
- Treats repository content as untrusted; validates all responses
- Requires explicit user confirmation for commits

### Provider Agnosticism
The `ports.LLM` interface allows swapping providers (OpenAI, Groq, etc.) without changing core logic.

## Roadmap

- [x] Foundation: project structure, interfaces, fakes
- [x] Domain & App Layer: models, validation, orchestrators
- [x] Adapters: git, redaction, cache, LLM clients
- [x] TUI: Bubble Tea UI with state machine
- [x] Config & CLI: environment variables and flags
- [x] Full integration test with real LLM (opt-in)
- [x] Persistent cache with `~/.cache/commit-coach/`
- [x] Hook installation for git workflows
- [x] Support for Gemini and Claude providers
- [x] Prompt customization via config file

## Contributing

Guidelines:
- All changes must pass `go test -race ./...`
- Add unit tests for new logic
- Use golden tests for prompt and output formatting

## License

MIT
