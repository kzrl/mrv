Project Overview

mrv is an autonomous CLI coding agent built in Go, leveraging the Agent Development Kit (ADK). It is designed to work with local models (via llama.cpp) to perform repository-level tasks: reading files, executing shell commands, and self-correcting via iterative loops.
# Build & Development

    Build binary: go build -o mrv main.go

    Run locally: ./mrv (Requires llama-server running on port 8080)

    Install dependencies: go mod tidy

    Linting: golangci-lint run
# Testing

    Run all tests: go test ./...

    Run specific package: go test ./internal/tools/...

    Verbose testing: go test -v ./...

    Integration tests: Integration tests requiring a live LLM are skipped by default; use -tags=integration to enable.

# Architecture & ADK Patterns

mrv follows the ADK-Go modular architecture:
1. The Agent (The Brain)

    Always use agent.NewLLMAgent wrapped in a LoopAgent for multi-step autonomy.

    Instructions: Kept in internal/agent/prompts.go. Do not hardcode system prompts in main.go.

2. Tools (The Hands)

    All tools must be defined in internal/tools/.

    Tools must use Strongly Typed Structs for arguments to allow ADK to auto-generate JSON schemas.

    Safety: Destructive tools (file deletion, git push) must require confirmation via agent.WithToolConfirmation(true).

3. Model Provider

    Uses github.com/huytd/adk-openai-go to bridge ADK with the local llama.cpp OpenAI-compatible server.

    Endpoint configuration is managed via environment variables: MRV_MODEL_URL and MRV_MODEL_NAME.

# Coding Standards

    Error Handling: Use fmt.Errorf("context: %w", err) for wrapping.

    Concurrency: Use context.Context throughout all tool definitions and agent runs.

    TUI: The UI uses Bubble Tea. Keep UI logic in internal/ui/ and agent logic in internal/agent/. Use Go channels/messages to communicate between the two.

    ADK Plugins: Always enable the RetryAndReflect plugin in the agent configuration to handle local model hallucinations/formatting errors.

# Project Structure
Plaintext

.
├── main.go             # Entry point & TUI initialization
├── go.mod              # Project dependencies
├── CLAUDE.md           # This guide
├── internal/
│   ├── agent/          # ADK Agent setup and loop logic
│   ├── tools/          # Tool definitions (shell, fs, lsp)
│   ├── ui/             # Bubble Tea TUI components (spinner, viewport)
│   └── model/          # llama.cpp / ADK adapter configuration
└── pkg/
    └── utils/          # Shared helper functions

# Key ADK Imports

    Logic: google.golang.org/adk/agent

    Tools: google.golang.org/adk/tool

    Local LLM: github.com/huytd/adk-openai-go
