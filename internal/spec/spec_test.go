package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashKnownVector(t *testing.T) {
	if got := Hash([]byte("abc")); got !=
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("Hash(abc) = %s", got)
	}
}

func TestReadEmptyFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "SPEC.md")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil {
		t.Fatal("Read of empty spec should fail")
	} else if msg := err.Error(); !strings.Contains(msg, "empty") {
		t.Fatalf("empty-spec error message %q should mention empty", msg)
	}
}

func TestReadRoundTrip(t *testing.T) {
	want := "# Demo\n\n## Intent\n\nDo the thing.\n"
	p := filepath.Join(t.TempDir(), "SPEC.md")
	if err := os.WriteFile(p, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("Read mutated contents: got %q, want %q", got, want)
	}
	if Hash(got) != Hash([]byte(want)) {
		t.Fatalf("Hash(Read) = %s, want Hash of original %s", Hash(got), Hash([]byte(want)))
	}
}

func TestReadMissingFails(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "SPEC.md")); err == nil {
		t.Fatal("Read of missing spec should fail")
	}
}
