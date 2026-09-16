// Package spec reads and fingerprints respex spec files.
package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// Skeleton is the spec file written by `respex init` without a description.
const Skeleton = `# <Project title>

## Intent

<What this is and why it exists.>

## Scope

<What is included.>

## Non-Goals

<What is explicitly out of scope.>

## Requirements

- <Behavioral requirement.>

## Open Questions

- <Unresolved decisions.>
`

// Read returns the spec file contents; empty files are an error.
func Read(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("spec %s is empty", path)
	}
	return b, nil
}

// Hash returns the sha256 hex digest of the contents.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
