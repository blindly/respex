package cli

import (
	"bytes"
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
