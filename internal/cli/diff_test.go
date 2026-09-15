package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDiffWorkingVsLastCommit(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\n\ntwo\n")
	var out bytes.Buffer
	if code := runDiff(nil, &out, &out); code != 0 {
		t.Fatalf("diff = %d, %s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "--- ") || !strings.Contains(s, "+++ ") ||
		!strings.Contains(s, "+two") {
		t.Fatalf("diff output missing hunks: %q", s)
	}
}

func TestDiffTwoVersions(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\n\ntwo\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out bytes.Buffer
	if code := runDiff([]string{"1", "2"}, &out, &out); code != 0 || !strings.Contains(out.String(), "+two") {
		t.Fatalf("diff v1 v2: %d, %s", code, out.String())
	}
}

func TestDiffNoCommits(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	var out bytes.Buffer
	if code := runDiff(nil, &out, &out); code != 1 {
		t.Fatal("diff before any commit must fail")
	}
}

func TestDiffBadArgs(t *testing.T) {
	setupProject(t)
	var out bytes.Buffer
	if code := runDiff([]string{"1"}, &out, &out); code != 1 {
		t.Fatal("one arg must fail")
	}
}
