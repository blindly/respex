package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// File is one member of a spec bundle: a repository-relative slash path and
// its contents. The first file of a bundle is always the master spec.
type File struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// CleanSpecPath validates and normalizes a config/proposal spec path. Paths
// are slash-separated in configuration regardless of platform. The returned
// path is relative, clean, and cannot escape the project root.
func CleanSpecPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty spec path")
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("spec path %q contains a NUL byte", p)
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || filepath.VolumeName(p) != "" {
		return "", fmt.Errorf("spec path %q must be relative to the project root", p)
	}
	clean := path.Clean(filepath.ToSlash(p))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("spec path %q escapes the project root", p)
	}
	return clean, nil
}

// ReadFiles reads each path under root into an ordered bundle. Paths are
// slash-separated; empty files are an error, same as Read.
func ReadFiles(root string, paths []string) ([]File, error) {
	files := make([]File, 0, len(paths))
	for _, p := range paths {
		content, err := Read(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: p, Content: content})
	}
	return files, nil
}

// HashFiles fingerprints a bundle over both paths and contents, so renames
// and reordering are detected alongside edits.
func HashFiles(files []File) string {
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write(f.Content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// EncodeFiles serializes a bundle for storage in spec_versions.content.
func EncodeFiles(files []File) []byte {
	b, err := json.Marshal(struct {
		Files []File `json:"files"`
	}{Files: files})
	if err != nil {
		panic(err) // File fields are always JSON-encodable
	}
	return b
}

// DecodeFiles interprets stored version content. The second return is false
// for legacy single-file rows (raw Markdown), true for encoded bundles.
func DecodeFiles(b []byte) ([]File, bool) {
	if !bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		return nil, false
	}
	var stored struct {
		Files []File `json:"files"`
	}
	if err := json.Unmarshal(b, &stored); err != nil || len(stored.Files) == 0 {
		return nil, false
	}
	for _, f := range stored.Files {
		if _, err := CleanSpecPath(f.Path); err != nil {
			return nil, false
		}
	}
	return stored.Files, true
}
