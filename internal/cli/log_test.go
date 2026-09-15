package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogOutput(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit([]string{"-m", "first"}, io.Discard, io.Discard)
	var out, errOut bytes.Buffer
	if code := runLog(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "v1") ||
		!strings.Contains(out.String(), "first") {
		t.Fatalf("log = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestLogRejectsArgs(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runLog([]string{"x"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "unexpected argument") {
		t.Fatalf("log args: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestStatusOutput(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, io.Discard, io.Discard)
	var out, errOut bytes.Buffer
	if code := runStatus(nil, &out, &errOut); code != 0 {
		t.Fatalf("status failed: %s | %s", out.String(), errOut.String())
	}
	for _, want := range []string{"SPEC.md", "dirty:       no", "last commit: v1", "applied:     no"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status missing %q in:\n%s", want, out.String())
		}
	}
}

func TestStatusDirtySpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, io.Discard, io.Discard)
	writeSpec(t, root, "# one\nchanged\n")
	var out, errOut bytes.Buffer
	if code := runStatus(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "dirty:       yes") {
		t.Fatalf("status dirty: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestStatusRejectsArgs(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runStatus([]string{"x"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "unexpected argument") {
		t.Fatalf("status args: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestStatusMissingSpecWithCommits(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, io.Discard, io.Discard)
	if err := os.Remove(filepath.Join(root, "SPEC.md")); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runStatus(nil, &out, &errOut); code != 0 {
		t.Fatalf("status missing spec: %d, %s | %s", code, out.String(), errOut.String())
	}
	for _, want := range []string{"dirty:       missing", "last commit: v1", "applied:     no"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status missing %q in:\n%s", want, out.String())
		}
	}
}

func TestLogAppliesRendering(t *testing.T) {
	root := setupProject(t)
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", \"marker\", \"{{prompt}}\"]\n", fakeBin))
	writeSpec(t, root, "# one\n")
	runCommit([]string{"-m", "first"}, io.Discard, io.Discard)
	var applyOut bytes.Buffer
	if code := runApply(nil, &applyOut, &applyOut); code != 0 {
		t.Fatalf("apply failed: %s", applyOut.String())
	}
	writeSpec(t, root, "# one\nchanged\n")
	runCommit([]string{"-m", "multi\nline"}, io.Discard, io.Discard)
	var out, errOut bytes.Buffer
	if code := runLog(nil, &out, &errOut); code != 0 {
		t.Fatalf("log = %d, %s | %s", code, out.String(), errOut.String())
	}
	s := out.String()
	for _, want := range []string{"applies:", "#1  v1  " + fakeBin + "  exit 0", ".respex/logs/1-apply.log", "multi line"} {
		if !strings.Contains(s, want) {
			t.Fatalf("log missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "multi\nline") {
		t.Fatalf("log renders raw newline in message:\n%s", s)
	}
	ai, vi := strings.Index(s, "applies:"), strings.Index(s, "versions:")
	if ai < 0 || vi < 0 || ai > vi {
		t.Fatalf("applies section must precede versions: %d vs %d in:\n%s", ai, vi, s)
	}
	vrest := s[vi:]
	if v2i, v1i := strings.Index(vrest, "  v2  "), strings.Index(vrest, "  v1  "); v2i < 0 || v1i < 0 || v2i > v1i {
		t.Fatalf("versions not newest-first in:\n%s", s)
	}
}

func TestLogEmpty(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runLog(nil, &out, &errOut); code != 0 {
		t.Fatalf("log empty: %d, %s | %s", code, out.String(), errOut.String())
	}
	for _, want := range []string{"versions:", "(none)"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("log empty missing %q in:\n%s", want, out.String())
		}
	}
}
