package ui

import (
	"io"
	"os"
	"regexp"
)

// IsTTY reports whether w is an interactive terminal.
func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

var (
	hunkRe = regexp.MustCompile(`(?m)^(@@.*@@.*)$`)
	addRe  = regexp.MustCompile(`(?m)^(\+.*$)`)
	delRe  = regexp.MustCompile(`(?m)^(-.*$)`)
)

const ansiReset = "\x1b[0m"

// ColorizeDiff adds ANSI colors to a unified diff: hunk headers cyan,
// additions green, deletions red.
func ColorizeDiff(diff string) string {
	out := hunkRe.ReplaceAllString(diff, "\x1b[36m$1"+ansiReset)
	out = addRe.ReplaceAllString(out, "\x1b[32m$1"+ansiReset)
	return delRe.ReplaceAllString(out, "\x1b[31m$1"+ansiReset)
}