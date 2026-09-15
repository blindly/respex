package cli

import (
	"bytes"
	"io"
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
