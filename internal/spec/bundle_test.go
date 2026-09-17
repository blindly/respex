package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFilesIncludesPath(t *testing.T) {
	a := []File{{Path: "SPEC.md", Content: []byte("# x\n")}, {Path: "specs/a.md", Content: []byte("a\n")}}
	b := []File{{Path: "SPEC.md", Content: []byte("# x\n")}, {Path: "specs/b.md", Content: []byte("a\n")}}
	if HashFiles(a) == HashFiles(b) {
		t.Fatal("hash must differ when only a path changes")
	}
	c := []File{{Path: "specs/a.md", Content: []byte("a\n")}, {Path: "SPEC.md", Content: []byte("# x\n")}}
	if HashFiles(a) == HashFiles(c) {
		t.Fatal("hash must differ when order changes")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	files := []File{
		{Path: "SPEC.md", Content: []byte("# master\n")},
		{Path: "specs/alpha.md", Content: []byte("# alpha\n")},
	}
	enc := EncodeFiles(files)
	got, ok := DecodeFiles(enc)
	if !ok {
		t.Fatal("DecodeFiles rejected encoded bundle")
	}
	if len(got) != 2 || got[0].Path != "SPEC.md" || string(got[1].Content) != "# alpha\n" {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestDecodeFilesLegacyMarkdown(t *testing.T) {
	if _, ok := DecodeFiles([]byte("# plain spec\n")); ok {
		t.Fatal("raw markdown must not decode as a bundle")
	}
	if _, ok := DecodeFiles([]byte(`{"files":[{"path":"../escape.md","content":"eQ=="}]}`)); ok {
		t.Fatal("bundle with escaping path must be rejected")
	}
	if _, ok := DecodeFiles([]byte(`{"files":[]}`)); ok {
		t.Fatal("empty bundle must not decode")
	}
}

func TestCleanSpecPath(t *testing.T) {
	for _, bad := range []string{"", "..", "../x.md", "/abs/x.md", "a/../../b.md", ".", "a\x00b"} {
		if _, err := CleanSpecPath(bad); err == nil {
			t.Fatalf("CleanSpecPath(%q) should fail", bad)
		}
	}
	for _, good := range []string{"SPEC.md", "specs/auth.md", "docs/spec/main.md"} {
		if _, err := CleanSpecPath(good); err != nil {
			t.Fatalf("CleanSpecPath(%q) = %v", good, err)
		}
	}
	if got, err := CleanSpecPath("specs//auth.md"); err != nil || got != "specs/auth.md" {
		t.Fatalf("CleanSpecPath normalization = %q, %v", got, err)
	}
}

func TestReadFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "SPEC.md"), []byte("# m\n"), 0o644)
	os.WriteFile(filepath.Join(root, "specs", "a.md"), []byte("# a\n"), 0o644)
	files, err := ReadFiles(root, []string{"SPEC.md", "specs/a.md"})
	if err != nil || len(files) != 2 || string(files[1].Content) != "# a\n" {
		t.Fatalf("ReadFiles = %+v, %v", files, err)
	}
	if _, err := ReadFiles(root, []string{"SPEC.md", "specs/missing.md"}); err == nil {
		t.Fatal("missing file must error")
	}
}
