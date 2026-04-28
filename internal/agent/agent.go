package agent

import (
	"context"
	"fmt"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/plugin/retryandreflect"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"

	"mrv/internal/config"
	mrvtools "mrv/internal/tools"
)

const (
	appName = "mrv"
	userID  = "local"
)

// Setup holds all runtime components needed to run the agent.
type Setup struct {
	Runner    *runner.Runner
	SessionID string
}

// New creates the full ADK agent stack:
// LLMAgent (with tools) → wrapped in LoopAgent → runner with RetryAndReflect plugin.
func New(llm adkmodel.LLM, cfg config.Config) (*Setup, error) {
	tools, err := buildTools(cfg.Tools)
	if err != nil {
		return nil, fmt.Errorf("agent: build tools: %w", err)
	}

	instruction := SystemPrompt
	if cfg.Agent.SystemPrompt != "" {
		instruction = cfg.Agent.SystemPrompt
	}

	llmAgent, err := llmagent.New(llmagent.Config{
		Name:        "mrv_llm",
		Description: "Autonomous coding agent that reads files, runs commands, and writes code.",
		Instruction: instruction,
		Model:       llm,
		Tools:       tools,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: create llm agent: %w", err)
	}

	loop, err := loopagent.New(loopagent.Config{
		AgentConfig: adkagent.Config{
			Name:      "mrv_loop",
			SubAgents: []adkagent.Agent{llmAgent},
		},
		MaxIterations: uint(cfg.Agent.MaxIterations),
	})
	if err != nil {
		return nil, fmt.Errorf("agent: create loop agent: %w", err)
	}

	rar, err := retryandreflect.New(
		retryandreflect.WithMaxRetries(cfg.Agent.MaxRetries),
	)
	if err != nil {
		return nil, fmt.Errorf("agent: create retryandreflect plugin: %w", err)
	}

	sessionSvc := session.InMemoryService()

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          loop,
		SessionService: sessionSvc,
		PluginConfig: runner.PluginConfig{
			Plugins: []*plugin.Plugin{rar},
		},
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: create runner: %w", err)
	}

	// Pre-create a session so we have a stable session ID across turns.
	ctx := context.Background()
	resp, err := sessionSvc.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("agent: create session: %w", err)
	}

	return &Setup{
		Runner:    r,
		SessionID: resp.Session.ID(),
	}, nil
}

// buildTools creates all ADK tools for the agent using config for confirmation flags.
func buildTools(cfg config.ToolsConfig) ([]tool.Tool, error) {
	shell, err := mrvtools.NewRunShellTool(cfg.Shell.RequireConfirmation)
	if err != nil {
		return nil, err
	}
	readFile, err := mrvtools.NewReadFileTool()
	if err != nil {
		return nil, err
	}
	editFile, err := mrvtools.NewEditFileTool()
	if err != nil {
		return nil, err
	}
	writeFile, err := mrvtools.NewWriteFileTool(cfg.WriteFile.RequireConfirmation)
	if err != nil {
		return nil, err
	}
	listFiles, err := mrvtools.NewListFilesTool()
	if err != nil {
		return nil, err
	}
	return []tool.Tool{shell, readFile, editFile, writeFile, listFiles}, nil
}
