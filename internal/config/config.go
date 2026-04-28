// Package config loads and validates mrv configuration from a CUE file.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema.cue
var schemaCUE string

// Config is the top-level mrv configuration.
type Config struct {
	Model ModelConfig `json:"model"`
	Agent AgentConfig `json:"agent"`
	Tools ToolsConfig `json:"tools"`
}

// ModelConfig holds LLM endpoint settings.
type ModelConfig struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// AgentConfig holds agent loop and retry settings.
type AgentConfig struct {
	MaxRetries    int    `json:"maxRetries"`
	MaxIterations int    `json:"maxIterations"`
	SystemPrompt  string `json:"systemPrompt"`
}

// ToolsConfig holds per-tool settings.
type ToolsConfig struct {
	Shell     ConfirmConfig `json:"shell"`
	WriteFile ConfirmConfig `json:"writeFile"`
}

// ConfirmConfig controls whether a tool requires user confirmation.
type ConfirmConfig struct {
	RequireConfirmation bool `json:"requireConfirmation"`
}

// Load finds, validates, and returns the effective configuration.
//
// Precedence (highest to lowest):
//  1. MRV_MODEL_URL / MRV_MODEL_NAME environment variables
//  2. ./mrv.cue (project-local)
//  3. $XDG_CONFIG_HOME/mrv/config.cue or ~/.config/mrv/config.cue
//  4. CUE schema defaults
func Load() (Config, error) {
	ctx := cuecontext.New()

	// Start from the embedded schema — this provides all defaults and constraints.
	value := ctx.CompileString(schemaCUE, cue.Filename("schema.cue"))
	if err := value.Err(); err != nil {
		return Config{}, fmt.Errorf("config: compile schema: %w", err)
	}

	// Overlay user config file if one exists.
	if path, ok := findConfigFile(); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("config: read %s: %w", path, err)
		}
		userVal := ctx.CompileBytes(data, cue.Filename(path))
		if err := userVal.Err(); err != nil {
			return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
		}
		value = value.Unify(userVal)
	}

	if err := value.Validate(cue.Concrete(true)); err != nil {
		return Config{}, fmt.Errorf("config: validation failed:\n%w", err)
	}

	var cfg Config
	if err := value.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: decode: %w", err)
	}

	// Environment variables override file config for backward compatibility.
	if v := os.Getenv("MRV_MODEL_URL"); v != "" {
		cfg.Model.URL = v
	}
	if v := os.Getenv("MRV_MODEL_NAME"); v != "" {
		cfg.Model.Name = v
	}

	return cfg, nil
}

// findConfigFile returns the path and true for the first config file found,
// searching: ./mrv.cue, then the XDG config directory.
func findConfigFile() (string, bool) {
	candidates := []string{
		"mrv.cue",
		xdgConfigPath(),
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// xdgConfigPath returns ~/.config/mrv/config.cue, respecting $XDG_CONFIG_HOME.
func xdgConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mrv", "config.cue")
}
