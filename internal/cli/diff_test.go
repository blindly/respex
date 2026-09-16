package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffWorkingVsLastCommit(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\n\ntwo\n")
	var out, errOut bytes.Buffer
	if code := runDiff(nil, &out, &errOut); code != 0 {
		t.Fatalf("diff = %d, %s", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "--- v1") || !strings.Contains(s, "+++ working") ||
		!strings.Contains(s, "+two") {
		t.Fatalf("diff output missing hunks: %q", s)
	}
	if strings.Contains(s, "\x1b[") {
		t.Fatalf("diff output must not be colorized to a non-TTY: %q", s)
	}
}

func TestDiffTwoVersions(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\n\ntwo\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"1", "2"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "+two") {
		t.Fatalf("diff v1 v2: %d, %s", code, errOut.String())
	}
}

func TestDiffAndRestoreRefinement(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# before\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"# after\\n\", \"{{prompt}}\"]\n", fakeBin))
	if code := runRefine(nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("refine exit = %d", code)
	}
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"--refine", "latest"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "-# before") || !strings.Contains(out.String(), "+# after") {
		t.Fatalf("refine diff = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runRestore([]string{"--refine", "latest", "--before"}, &out, &errOut); code != 0 {
		t.Fatalf("restore = %d, %s | %s", code, out.String(), errOut.String())
	}
	content, err := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if err != nil || string(content) != "# before\n" {
		t.Fatalf("restored content = %q, %v", content, err)
	}
	if code := runRestore([]string{"--refine", "latest", "--before"}, &out, &errOut); code != 0 {
		t.Fatalf("reverse restore = %d, %s | %s", code, out.String(), errOut.String())
	}
	content, err = os.ReadFile(filepath.Join(root, "SPEC.md"))
	if err != nil || string(content) != "# after\n" {
		t.Fatalf("reverse restored content = %q, %v", content, err)
	}
}

func TestDiffNoDifferences(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"1", "1"}, &out, &errOut); code != 0 || out.String() != "no differences\n" {
		t.Fatalf("diff v1 v1 = %d, out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestDiffNoCommits(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	var out, errOut bytes.Buffer
	if code := runDiff(nil, &out, &errOut); code != 1 || errOut.String() == "" || out.String() != "" {
		t.Fatal("diff before any commit must fail")
	}
}

func TestDiffBadArgs(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"1"}, &out, &errOut); code != 1 || errOut.String() == "" || out.String() != "" {
		t.Fatal("one arg must fail")
	}
}

func TestDiffNonIntegerArgs(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"a", "b"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "usage") || out.String() != "" {
		t.Fatalf("non-integer args: %d, out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestDiffMissingVersion(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runDiff([]string{"1", "99"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no such version v99") || out.String() != "" {
		t.Fatalf("missing version: %d, out=%q err=%q", code, out.String(), errOut.String())
	}
}
