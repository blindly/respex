package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestColorizeDiff(t *testing.T) {
	in := "--- a\n+++ b\n@@ -1 +1 @@\n-old\n+new\n ctx\n"
	want := "\x1b[31m--- a\x1b[0m\n\x1b[32m+++ b\x1b[0m\n" +
		"\x1b[36m@@ -1 +1 @@\x1b[0m\n\x1b[31m-old\x1b[0m\n\x1b[32m+new\x1b[0m\n ctx\n"
	if got := ColorizeDiff(in); got != want {
		t.Fatalf("ColorizeDiff mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestColorizeDiffCommaHunk(t *testing.T) {
	in := "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-old1\n-old2\n+new1\n+new2\n"
	want := "\x1b[31m--- a\x1b[0m\n\x1b[32m+++ b\x1b[0m\n" +
		"\x1b[36m@@ -1,2 +1,2 @@\x1b[0m\n\x1b[31m-old1\x1b[0m\n\x1b[31m-old2\x1b[0m\n" +
		"\x1b[32m+new1\x1b[0m\n\x1b[32m+new2\x1b[0m\n"
	if got := ColorizeDiff(in); got != want {
		t.Fatalf("ColorizeDiff comma-hunk mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestColorizeDiffEmpty(t *testing.T) {
	if got := ColorizeDiff(""); got != "" {
		t.Fatalf("ColorizeDiff(empty) = %q, want empty", got)
	}
}

func TestProgress(t *testing.T) {
	var out bytes.Buffer
	progress := StartProgress(&out, "applying v1", true)
	time.Sleep(120 * time.Millisecond)
	progress.Stop()
	progress.Stop()
	if text := out.String(); !strings.Contains(text, "applying v1") || !strings.Contains(text, "\r") {
		t.Fatalf("progress output = %q", text)
	}
}

func TestProgressDisabled(t *testing.T) {
	var out bytes.Buffer
	progress := StartProgress(&out, "applying v1", false)
	progress.Stop()
	if out.Len() != 0 {
		t.Fatalf("disabled progress output = %q", out.String())
	}
}

func TestIsTTYNonTTY(t *testing.T) {
	if IsTTY(io.Discard) {
		t.Fatal("non-*os.File writer reported as TTY")
	}
}
