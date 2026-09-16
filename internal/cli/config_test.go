package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	respexconfig "github.com/blindly/respex/internal/config"
)

func TestConfigInitAndPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	var out, errOut bytes.Buffer
	if code := runConfig([]string{"init"}, &out, &errOut); code != 0 {
		t.Fatalf("config init = %d, %s | %s", code, out.String(), errOut.String())
	}
	path := filepath.Join(dir, "respex", "config.toml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"claude", "devin", "codex", "gemini", "opencode"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("config template missing %q", want)
		}
	}
	out.Reset()
	if code := runConfig([]string{"path"}, &out, &errOut); code != 0 || strings.TrimSpace(out.String()) != path {
		t.Fatalf("config path = %d, %q | %s", code, out.String(), errOut.String())
	}
	if code := runConfig([]string{"init"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "already exists") {
		t.Fatalf("second config init = %d, %s", code, errOut.String())
	}
}

func TestConfigLocalPathEditAndValidate(t *testing.T) {
	root := setupProject(t)
	path := filepath.Join(root, ".respex", "config.toml")
	updated := "editor = [\"code\", \"--wait\"]\n[agent]\ncommand = [\"claude\", \"-p\", \"{{prompt}}\"]\n"
	writeConfig(t, root, fmt.Sprintf("editor = [%q, \"-write\", %q, \"-content\", %q]\n", fakeBin, path, updated))
	var out, errOut bytes.Buffer
	if code := runConfig([]string{"path", "--local"}, &out, &errOut); code != 0 || strings.TrimSpace(out.String()) != path {
		t.Fatalf("local path = %d, %q | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := runConfig([]string{"edit", "--local"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "updated project config") {
		t.Fatalf("local edit = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := runConfig([]string{"validate", "--local"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "is valid") {
		t.Fatalf("local validate = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestConfigShowSourcesAndRedactsEnv(t *testing.T) {
	root := setupProject(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	global := filepath.Join(dir, "respex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(global), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("agent_timeout = \"2h\"\n[agent]\nenv = [\"TOKEN=secret\"]\ncommand = [\"devin\", \"--print\", \"{{prompt}}\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, "[agent]\ncommand = [\"claude\", \"-p\", \"{{prompt}}\"]\n")
	var out, errOut bytes.Buffer
	if code := runConfig([]string{"show"}, &out, &errOut); code != 0 {
		t.Fatalf("config show = %d, %s | %s", code, out.String(), errOut.String())
	}
	for _, want := range []string{"agent.command:", "(project)", "agent_timeout: 2h0m0s  (user)", "TOKEN=<redacted>"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("config show missing %q in %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "secret") {
		t.Fatalf("config show leaked secret: %s", out.String())
	}
}

func TestProjectAgentOverridesUserAgent(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.toml")
	project := filepath.Join(dir, "project.toml")
	if err := os.WriteFile(global, []byte("[agent]\ncommand = [\"devin\", \"--print\", \"{{prompt}}\"]\ndelivery = \"argv\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project, []byte("[agent]\ncommand = [\"claude\", \"-p\", \"{{prompt}}\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := respexconfig.Load(global, project)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Command[0] != "claude" || cfg.Agent.Delivery != "argv" {
		t.Fatalf("merged agent = %+v", cfg.Agent)
	}
}
