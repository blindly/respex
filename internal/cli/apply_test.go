package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestApplyDirtySpecFails(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\nchanged\n")
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "spec changed since last commit") {
		t.Fatalf("apply dirty: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyRequiresAgent(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no agent configured") {
		t.Fatalf("apply without agent: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyNoCommits(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no committed versions yet") {
		t.Fatalf("apply without commit: %d, %s | %s", code, out.String(), errOut.String())
	}
}
