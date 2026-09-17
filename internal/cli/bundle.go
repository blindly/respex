package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blindly/respex/internal/spec"
)

// specPaths returns the ordered bundle paths — the master spec first, then
// configured spec_files — as validated repository-relative slash paths.
func (w *workspace) specPaths() ([]string, error) {
	master, err := spec.CleanSpecPath(w.cfg.Spec)
	if err != nil {
		return nil, err
	}
	paths := []string{master}
	seen := map[string]bool{master: true}
	for _, p := range w.cfg.SpecFiles {
		clean, err := spec.CleanSpecPath(p)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			return nil, fmt.Errorf("spec path %q is listed more than once", clean)
		}
		seen[clean] = true
		paths = append(paths, clean)
	}
	return paths, nil
}

// readSpecFiles reads every configured spec file into an ordered bundle.
func (w *workspace) readSpecFiles() ([]spec.File, error) {
	paths, err := w.specPaths()
	if err != nil {
		return nil, err
	}
	return spec.ReadFiles(w.root, paths)
}

// bundleHash matches the encoding stored in spec_versions.hash: a raw content
// hash for single-file specs, a path-aware bundle hash for multi-file specs.
func bundleHash(files []spec.File) string {
	if len(files) == 1 {
		return spec.Hash(files[0].Content)
	}
	return spec.HashFiles(files)
}

// workingSpecHash fingerprints the live spec bundle the same way commit does.
func (w *workspace) workingSpecHash() (string, error) {
	files, err := w.readSpecFiles()
	if err != nil {
		return "", err
	}
	return bundleHash(files), nil
}

// storedSpecContent encodes a bundle for spec_versions.content. Single-file
// specs keep the legacy raw-Markdown encoding so existing history stays
// comparable; multi-file bundles are stored as JSON.
func storedSpecContent(files []spec.File) ([]byte, string) {
	if len(files) == 1 {
		return files[0].Content, spec.Hash(files[0].Content)
	}
	return spec.EncodeFiles(files), spec.HashFiles(files)
}

// versionFiles decodes stored version content into an ordered bundle. Legacy
// rows hold raw Markdown for the master spec only.
func versionFiles(content []byte, master string) []spec.File {
	if files, ok := spec.DecodeFiles(content); ok {
		return files
	}
	return []spec.File{{Path: master, Content: content}}
}

// materializeSnapshot writes a committed spec bundle under .respex/tmp with
// read-only files and returns the absolute master-spec path for the agent.
func materializeSnapshot(root string, versionID int64, files []spec.File) (string, func(), error) {
	if len(files) == 1 {
		p, cleanup, err := createSpecCandidate(root, files[0].Content)
		if err != nil {
			return "", nil, err
		}
		if err := os.Chmod(p, 0o444); err != nil {
			cleanup()
			return "", nil, err
		}
		return p, cleanup, nil
	}
	tmpParent := filepath.Join(root, ".respex", "tmp")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return "", nil, err
	}
	tmpDir, err := os.MkdirTemp(tmpParent, fmt.Sprintf("apply-v%d-*", versionID))
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	for _, f := range files {
		clean, err := spec.CleanSpecPath(f.Path)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		dest := filepath.Join(tmpDir, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			cleanup()
			return "", nil, err
		}
		if err := os.WriteFile(dest, f.Content, 0o444); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return filepath.Join(tmpDir, filepath.FromSlash(files[0].Path)), cleanup, nil
}
