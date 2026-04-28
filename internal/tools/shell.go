// Package tools provides ADK tool definitions for the mrv agent.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// RunShellArgs are the input arguments for the RunShell tool.
type RunShellArgs struct {
	Command string `json:"command" jsonschema_description:"Shell command to execute (runs via /bin/sh -c)"`
}

// RunShellResult is the output of the RunShell tool.
type RunShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// NewRunShellTool creates a tool that executes shell commands.
// requireConfirmation controls whether the agent must ask before running.
func NewRunShellTool(requireConfirmation bool) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:                "run_shell",
		Description:         "Execute a shell command and return stdout, stderr, and exit code. Use for running tests, builds, git commands, etc.",
		RequireConfirmation: requireConfirmation,
	}, runShell)
}

func runShell(_ tool.Context, args RunShellArgs) (RunShellResult, error) {
	if args.Command == "" {
		return RunShellResult{}, fmt.Errorf("command must not be empty")
	}

	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", args.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return RunShellResult{}, fmt.Errorf("run command: %w", err)
		}
	}

	return RunShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}
