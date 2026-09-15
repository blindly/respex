package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("APPDATA", cfg)
	t.Chdir(root)
	return root
}

func TestDiscoverFindsNearestRespex(t *testing.T) {
	root := isolate(t)
	respexDir := filepath.Join(root, ".respex")
	if err := os.MkdirAll(respexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(respexDir, "config.toml"), []byte("spec = \"OTHER.md\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	w, err := discover()
	if err != nil {
		t.Fatal(err)
	}
	if w.root != root {
		t.Fatalf("root = %s, want %s", w.root, root)
	}
	if w.cfg.Spec != "OTHER.md" {
		t.Fatalf("cfg.Spec = %q, want OTHER.md", w.cfg.Spec)
	}
	if !strings.HasSuffix(w.specPath(), "OTHER.md") {
		t.Fatalf("specPath = %s, want suffix OTHER.md", w.specPath())
	}
}

func TestDiscoverFailsWithoutProject(t *testing.T) {
	isolate(t)
	if _, err := discover(); !errors.Is(err, errNotProject) {
		t.Fatalf("err = %v, want errNotProject", err)
	}
}

func TestDiscoverPrefersNearestRespex(t *testing.T) {
	root := isolate(t)
	if err := os.MkdirAll(filepath.Join(root, ".respex"), 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(filepath.Join(inner, ".respex"), 0o755); err != nil {
		t.Fatal(err)
	}
	deeper := filepath.Join(inner, "deeper")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deeper)
	w, err := discover()
	if err != nil {
		t.Fatal(err)
	}
	if w.root != inner {
		t.Fatalf("root = %s, want inner %s", w.root, inner)
	}
}
