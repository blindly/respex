package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupProject(t *testing.T) string {
	root := isolate(t)
	if err := os.MkdirAll(filepath.Join(root, ".respex"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeSpec(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "SPEC.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommitVersions(t *testing.T) {
	root := setupProject(t)
	var out bytes.Buffer
	writeSpec(t, root, "# one\n")
	if code := runCommit([]string{"-m", "first"}, &out, &out); code != 0 {
		t.Fatalf("commit = %d, %s", code, out.String())
	}
	if !strings.Contains(out.String(), "committed v1") {
		t.Fatalf("out = %s", out.String())
	}
	writeSpec(t, root, "# one\n\n## more\n")
	out.Reset()
	if code := runCommit(nil, &out, &out); code != 0 || !strings.Contains(out.String(), "committed v2") {
		t.Fatalf("commit2 = %d, %s", code, out.String())
	}
}

func TestCommitIdenticalWarns(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	var out bytes.Buffer
	if code := runCommit(nil, &out, &out); code != 0 {
		t.Fatal("first commit failed")
	}
	out.Reset()
	if code := runCommit(nil, &out, &out); code != 0 || !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("identical commit: %d, %s", code, out.String())
	}
}

func TestCommitMissingSpec(t *testing.T) {
	setupProject(t)
	var out bytes.Buffer
	if code := runCommit(nil, &out, &out); code != 1 {
		t.Fatal("commit without spec must fail")
	}
}
