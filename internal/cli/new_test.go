package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScaffold(t *testing.T) {
	root := isolate(t)
	var out bytes.Buffer
	if code := runNew(nil, &out, &out); code != 0 {
		t.Fatalf("runNew = %d, out=%s", code, out.String())
	}
	for _, p := range []string{".respex/config.toml", ".respex/state.db", "SPEC.md"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	body, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if !strings.Contains(string(body), "## Intent") {
		t.Fatalf("skeleton wrong: %q", body)
	}
	if code := runNew(nil, &out, &out); code != 1 {
		t.Fatal("second new must fail")
	}
}

func TestNewRefusesExistingSpec(t *testing.T) {
	root := isolate(t)
	if err := os.WriteFile(filepath.Join(root, "SPEC.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runNew(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "SPEC.md") || out.String() != "" {
		t.Fatalf("runNew = %d, out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestNewAddsGitignore(t *testing.T) {
	root := isolate(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := runNew(nil, &out, &out); code != 0 {
		t.Fatal("new should succeed")
	}
	g, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.HasPrefix(string(g), ".respex/") {
		t.Fatalf(".gitignore = %q (fresh file must not get a leading blank line)", g)
	}
}

func TestNewNoAgentWithDescription(t *testing.T) {
	root := isolate(t)
	var out, errOut bytes.Buffer
	if code := runNew([]string{"an idea"}, &out, &errOut); code != 0 {
		t.Fatalf("runNew = %d, out=%q err=%q", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if !strings.Contains(string(body), "## Intent") {
		t.Fatalf("skeleton not written: %q", body)
	}
	if s := out.String(); !strings.Contains(s, "no agent configured") || !strings.Contains(s, "respex refine") {
		t.Fatalf("out = %q", s)
	}
}

func TestNewPreservesExistingConfig(t *testing.T) {
	root := isolate(t)
	if err := os.MkdirAll(filepath.Join(root, ".respex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, ".respex", "config.toml")
	cfgBody := "spec = \"OTHER.md\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runNew(nil, &out, &errOut); code != 0 {
		t.Fatalf("runNew = %d, out=%q err=%q", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(filepath.Join(root, "OTHER.md"))
	if !strings.Contains(string(body), "## Intent") {
		t.Fatalf("OTHER.md not skeleton: %q", body)
	}
	after, _ := os.ReadFile(cfgPath)
	if string(after) != cfgBody {
		t.Fatalf("config.toml rewritten: %q", after)
	}
}
