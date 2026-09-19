package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestPromptsMergeKeyByKey(t *testing.T) {
	dir := t.TempDir()
	g := filepath.Join(dir, "global.toml")
	p := filepath.Join(dir, "project.toml")
	write(t, g, `[prompts]
refine = "global-refine"
apply = "global-apply"
verify = "global-verify"
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
	if cfg.Prompts.Verify != "global-verify" {
		t.Fatalf("prompts verify merge wrong: %q", cfg.Prompts.Verify)
	}
}

func TestAgentTimeout(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")
	write(t, global, `apply_timeout = "2h"`)
	write(t, project, `agent_timeout = "30m"`)
	cfg, err := Load(global, project)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentTimeout != 30*time.Minute {
		t.Fatalf("agent timeout = %s", cfg.AgentTimeout)
	}
	write(t, project, `apply_timeout = "never"`)
	if _, err := Load(global, project); err == nil || !strings.Contains(err.Error(), "invalid agent timeout") {
		t.Fatalf("want invalid agent timeout error, got %v", err)
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

func TestProjectWinsTopLevel(t *testing.T) {
	dir := t.TempDir()
	g := filepath.Join(dir, "global.toml")
	p := filepath.Join(dir, "project.toml")
	write(t, g, `spec = "GLOBAL.md"`)
	write(t, p, `spec = "PROJECT.md"`)
	cfg, err := Load(g, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Spec != "PROJECT.md" {
		t.Fatalf("spec = %q, want PROJECT.md", cfg.Spec)
	}
}

func TestReadErrorSurfaces(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, ""); err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("want read config error, got %v", err)
	}
}

func TestGlobalPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG_CONFIG_HOME is not honored on Windows")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "respex", "config.toml"); got != want {
		t.Fatalf("GlobalPath() = %q, want %q", got, want)
	}
}

func TestVerifyMerge(t *testing.T) {
	dir := t.TempDir()
	g := filepath.Join(dir, "global.toml")
	p := filepath.Join(dir, "project.toml")
	write(t, g, `[verify]
commands = [["go", "test", "./..."]]
timeout = "1h"
audit = true
`)
	write(t, p, `[verify]
commands = [["go", "build", "./..."]]
env = ["GOFLAGS=-count=1"]
`)
	cfg, err := Load(g, p)
	if err != nil {
		t.Fatal(err)
	}
	// commands and env come from project; timeout survives from global.
	if len(cfg.Verify.Commands) != 1 || cfg.Verify.Commands[0][0] != "go" || cfg.Verify.Commands[0][1] != "build" {
		t.Fatalf("verify commands merge wrong: %+v", cfg.Verify.Commands)
	}
	if cfg.Verify.Timeout != time.Hour {
		t.Fatalf("verify timeout = %s; want 1h from global", cfg.Verify.Timeout)
	}
	if len(cfg.Verify.Env) != 1 || cfg.Verify.Env[0] != "GOFLAGS=-count=1" {
		t.Fatalf("verify env merge wrong: %+v", cfg.Verify.Env)
	}
	// audit = true survives from global when project does not set it.
	if !cfg.Verify.Audit {
		t.Fatal("verify audit merge wrong: want true from global")
	}
}

func TestVerifyDefaultsAndInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "nope.toml"), filepath.Join(dir, "nope2.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Verify.Timeout != 30*time.Minute || len(cfg.Verify.Commands) != 0 {
		t.Fatalf("verify defaults wrong: %+v", cfg.Verify)
	}
	p := filepath.Join(dir, "bad.toml")
	write(t, p, `[verify]
timeout = "never"
`)
	if _, err := Load("", p); err == nil || !strings.Contains(err.Error(), "invalid verify timeout") {
		t.Fatalf("want invalid verify timeout error, got %v", err)
	}
}
