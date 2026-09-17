package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type specQuestion struct {
	File string `json:"file"`
	Text string `json:"text"`
}

var topLevelItemRe = regexp.MustCompile(`^(\s*)((?:\d+\.)|[-*])\s+(.*)$`)

// extractQuestions returns the bullet/numbered items under the first
// "## Open Questions" heading in content. Continuation lines that belong to the
// same item are included; nested sub-items are flattened into the parent item.
func extractQuestions(content []byte) []string {
	var questions []string
	var current strings.Builder
	inSection := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		text := strings.TrimSpace(current.String())
		if text != "" {
			questions = append(questions, collapseWhitespace(text))
		}
		current.Reset()
	}

	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "## open questions") {
			flush()
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			flush()
			break
		}
		if !inSection {
			continue
		}
		if trimmed == "" {
			continue
		}
		if topLevelItemRe.MatchString(line) {
			flush()
			matches := topLevelItemRe.FindStringSubmatch(line)
			current.WriteString(matches[3])
		} else if current.Len() > 0 {
			// Continuation of the current item.
			if current.Len() > 0 && !strings.HasSuffix(current.String(), " ") {
				current.WriteString(" ")
			}
			current.WriteString(trimmed)
		}
	}
	flush()
	return questions
}

func collapseWhitespace(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

func runSpecQuestions(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("spec questions", errOut)
	jsonOut := fs.Bool("json", false, "output questions as JSON")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("usage: respex spec questions [--json]"))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	files, err := w.readSpecFiles()
	if err != nil {
		return fail(errOut, err)
	}

	var all []specQuestion
	for _, f := range files {
		for _, q := range extractQuestions(f.Content) {
			all = append(all, specQuestion{File: f.Path, Text: q})
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return boolToInt(enc.Encode(map[string]any{"questions": all, "count": len(all)}) != nil)
	}

	if len(all) == 0 {
		fmt.Fprintln(out, "no open questions found")
		return 0
	}

	currentFile := ""
	for _, q := range all {
		if q.File != currentFile {
			currentFile = q.File
			fmt.Fprintf(out, "\n%s\n\n", currentFile)
		}
		fmt.Fprintf(out, "- %s\n", q.Text)
	}
	fmt.Fprintf(out, "\n%d open question(s)\n", len(all))
	return 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
