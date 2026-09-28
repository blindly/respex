// Package spec reads and fingerprints respex spec files.
package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(?:\d+[.)]\s+)?(.*)$`)

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

// HeadingText returns the normalized text of a Markdown heading at the given
// level, ignoring optional leading numbers such as `## 1. Intent`. It returns an
// empty string if the line is not a heading of the requested level.
func HeadingText(line string, level int) string {
	matches := headingRe.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 3 || len(matches[1]) != level {
		return ""
	}
	return strings.TrimSpace(matches[2])
}

func MissingSections(content []byte) []string {
	required := []string{"Intent", "Scope", "Non-Goals", "Requirements", "Open Questions"}
	found := make(map[string]bool, len(required))
	for _, line := range strings.Split(string(content), "\n") {
		text := HeadingText(line, 2)
		if text == "" {
			text = HeadingText(line, 1)
		}
		if text == "" {
			continue
		}
		norm := normalizeSection(text)
		for _, section := range required {
			if norm == normalizeSection(section) {
				found[section] = true
				break
			}
		}
		// Headings like "Intent: why this exists" still count.
		if norm == "" {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(text))
		for _, section := range required {
			ls := strings.ToLower(section)
			if strings.HasPrefix(lower, ls+":") || strings.HasPrefix(lower, ls+" -") || strings.HasPrefix(lower, ls+" –") || strings.HasPrefix(lower, ls+" —") || strings.HasPrefix(lower, ls+" (") {
				found[section] = true
				break
			}
		}
	}
	var missing []string
	for _, section := range required {
		if !found[section] {
			missing = append(missing, section)
		}
	}
	return missing
}

// normalizeSection lowercases a heading and strips everything except a-z so
// "Non-Goals", "Non Goals", and "nongoals" compare equal. Trailing colons
// like "Intent:" are ignored.
func normalizeSection(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimRight(s, ":")
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
