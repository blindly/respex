package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blindly/respex/internal/spec"
)

func createSpecCandidate(root string, content []byte) (string, func(), error) {
	dir := filepath.Join(root, ".respex", "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(dir, "spec-*.md")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := f.Write(content); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func installSpecCandidate(livePath, candidatePath string, expectedLive []byte) ([]byte, error) {
	current, err := os.ReadFile(livePath)
	if expectedLive == nil {
		if err == nil {
			return nil, fmt.Errorf("spec changed while the agent was running")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, fmt.Errorf("spec changed while the agent was running: %w", err)
		}
		if spec.Hash(current) != spec.Hash(expectedLive) {
			return nil, fmt.Errorf("spec changed while the agent was running; candidate was not installed")
		}
	}
	content, err := spec.Read(candidatePath)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(livePath), ".respex-spec-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := replaceFile(tmpPath, livePath); err != nil {
		return nil, err
	}
	return content, nil
}
