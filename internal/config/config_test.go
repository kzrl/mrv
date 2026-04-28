package config_test

import (
	"os"
	"testing"
	"mrv/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Model.URL != "http://localhost:8080/v1" {
		t.Errorf("URL = %q, want default", cfg.Model.URL)
	}
	if cfg.Model.Name != "local-model" {
		t.Errorf("Name = %q, want default", cfg.Model.Name)
	}
	if cfg.Agent.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.Agent.MaxRetries)
	}
	if cfg.Agent.MaxIterations != 0 {
		t.Errorf("MaxIterations = %d, want 0", cfg.Agent.MaxIterations)
	}
	if cfg.Tools.Shell.RequireConfirmation {
		t.Error("Shell.RequireConfirmation should default to false")
	}
	if cfg.Tools.WriteFile.RequireConfirmation {
		t.Error("WriteFile.RequireConfirmation should default to false")
	}
}

func TestFileOverride(t *testing.T) {
	// Write a local mrv.cue that overrides model name and disables shell confirm.
	const cue = `
model: name: "test-model"
agent: maxRetries: 5
tools: shell: requireConfirmation: false
`
	if err := os.WriteFile("mrv.cue", []byte(cue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove("mrv.cue") })

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Model.Name != "test-model" {
		t.Errorf("Name = %q, want test-model", cfg.Model.Name)
	}
	if cfg.Agent.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.Agent.MaxRetries)
	}
	if cfg.Tools.Shell.RequireConfirmation {
		t.Error("Shell.RequireConfirmation should be false after override")
	}
	// writeFile confirm unchanged by override — should still be the default (false)
	if cfg.Tools.WriteFile.RequireConfirmation {
		t.Error("WriteFile.RequireConfirmation should still be false (schema default)")
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("MRV_MODEL_URL", "http://custom:9999/v1")
	t.Setenv("MRV_MODEL_NAME", "env-model")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Model.URL != "http://custom:9999/v1" {
		t.Errorf("URL = %q, want env override", cfg.Model.URL)
	}
	if cfg.Model.Name != "env-model" {
		t.Errorf("Name = %q, want env override", cfg.Model.Name)
	}
}

func TestInvalidConfig(t *testing.T) {
	const cue = `agent: maxRetries: -1`
	if err := os.WriteFile("mrv.cue", []byte(cue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove("mrv.cue") })

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected validation error for maxRetries: -1")
	}
}
