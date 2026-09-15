package spec

import (
	"os"
	"path/filepath"
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
	}
}

func TestReadMissingFails(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "SPEC.md")); err == nil {
		t.Fatal("Read of missing spec should fail")
	}
}
