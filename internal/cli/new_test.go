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
	var out bytes.Buffer
	if code := runNew(nil, &out, &out); code != 1 || !strings.Contains(out.String(), "SPEC.md") {
		t.Fatalf("runNew = %d, out=%s", code, out.String())
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
	if !strings.Contains(string(g), ".respex/") {
		t.Fatalf(".gitignore = %q", g)
	}
}
