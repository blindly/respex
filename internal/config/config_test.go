package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMissingFilesYieldDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "nope.toml"), filepath.Join(dir, "nope2.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec != "SPEC.md" || cfg.Agent.Delivery != "argv" || len(cfg.Agent.Command) != 0 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
}

func TestProjectOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	g := filepath.Join(dir, "global.toml")
	p := filepath.Join(dir, "project.toml")
	write(t, g, `
spec = "OTHER.md"
[agent]
command = ["claude", "-p", "{{prompt}}"]
delivery = "argv"
env = ["A=1"]
`)
	write(t, p, `
[agent]
command = ["gemini", "-p", "{{prompt}}"]
`)
	cfg, err := Load(g, p)
	if err != nil {
		t.Fatal(err)
	}
	// Top-level: project file has no spec key → global value survives.
	if cfg.Spec != "OTHER.md" {
		t.Fatalf("spec = %q", cfg.Spec)
	}
	// [agent]: command and env come from project, delivery survives from global.
	if cfg.Agent.Command[0] != "gemini" || cfg.Agent.Delivery != "argv" || cfg.Agent.Env[0] != "A=1" {
		t.Fatalf("agent merge wrong: %+v", cfg.Agent)
	}
}

func TestPromptsMergeKeyBykey(t *testing.T) {
	dir := t.TempDir()
	g := filepath.Join(dir, "global.toml")
	p := filepath.Join(dir, "project.toml")
	write(t, g, `[prompts]
refine = "global-refine"
apply = "global-apply"
`)
	write(t, p, `[prompts]
apply = "project-apply"
`)
	cfg, err := Load(g, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prompts.Refine != "global-refine" || cfg.Prompts.Apply != "project-apply" || cfg.Prompts.Draft != "" {
		t.Fatalf("prompts merge wrong: %+v", cfg.Prompts)
	}
}

func TestInvalidTOMLIsAnError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.toml")
	write(t, p, "spec = ")
	if _, err := Load("", p); err == nil {
		t.Fatal("parse error should surface")
	}
}
