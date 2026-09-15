package ui

import "testing"

func TestColorizeDiff(t *testing.T) {
	in := "--- a\n+++ b\n@@ -1 +1 @@\n-old\n+new\n ctx\n"
	want := "\x1b[31m--- a\x1b[0m\n\x1b[32m+++ b\x1b[0m\n" +
		"\x1b[36m@@ -1 +1 @@\x1b[0m\n\x1b[31m-old\x1b[0m\n\x1b[32m+new\x1b[0m\n ctx\n"
	if got := ColorizeDiff(in); got != want {
		t.Fatalf("ColorizeDiff mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestIsTTYNonTTY(t *testing.T) {
	if IsTTY(&discard{}) {
		t.Fatal("bytes.Buffer-backed writer reported as TTY")
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }