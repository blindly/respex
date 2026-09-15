package cli

import (
	"os"
	"path/filepath"
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
	if err := os.MkdirAll(filepath.Join(root, ".respex"), 0o755); err != nil {
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
	if w.cfg.Spec != "SPEC.md" {
		t.Fatalf("cfg.Spec = %q", w.cfg.Spec)
	}
}

func TestDiscoverFailsWithoutProject(t *testing.T) {
	isolate(t)
	if _, err := discover(); err == nil {
		t.Fatal("discover outside a project should fail")
	}
}
