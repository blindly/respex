package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/blindly/respex/internal/spec"
)

type checkResult struct {
	Check   string `json:"check"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type checkSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

func addResult(results *[]checkResult, summary *checkSummary, level, check, msg string) {
	*results = append(*results, checkResult{Check: check, Level: level, Message: msg})
	switch level {
	case "pass":
		summary.Pass++
	case "warn":
		summary.Warn++
	case "fail":
		summary.Fail++
	}
}

var mdLinkRe = regexp.MustCompile(`(?m)\[([^\]]+)\]\(([^)]+)\)`)

func runCheck(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("check", errOut)
	jsonOut := fs.Bool("json", false, "output results as JSON")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex check [--json]", fs.Arg(0)))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	paths, err := w.specPaths()
	if err != nil {
		return fail(errOut, err)
	}

	var results []checkResult
	var summary checkSummary

	// Build the set of configured spec paths and feature basenames.
	configured := make(map[string]bool, len(paths))
	features := make(map[string]string) // basename(.md removed) -> path
	for _, p := range paths {
		configured[p] = true
		if p != w.cfg.Spec {
			base := path.Base(p)
			if strings.HasSuffix(base, ".md") {
				features[strings.TrimSuffix(base, ".md")] = p
			}
		}
	}

	// Load file contents.
	contents := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := spec.Read(p)
		if err != nil {
			addResult(&results, &summary, "fail", "readable", fmt.Sprintf("%s: %v", p, err))
			continue
		}
		if len(bytes.TrimSpace(b)) == 0 {
			addResult(&results, &summary, "fail", "non-empty", fmt.Sprintf("%s: file is empty", p))
			continue
		}
		contents[p] = b
		addResult(&results, &summary, "pass", "readable", fmt.Sprintf("%s: readable (%d bytes)", p, len(b)))
	}

	// Required-sections checks for every configured spec file.
	for p, b := range contents {
		missing := spec.MissingSections(b)
		if len(missing) == 0 {
			addResult(&results, &summary, "pass", "required-sections", fmt.Sprintf("%s contains required sections", p))
		} else {
			level := "fail"
			if p != w.cfg.Spec {
				level = "warn"
			}
			addResult(&results, &summary, level, "required-sections", fmt.Sprintf("%s missing sections: %s", p, strings.Join(missing, ", ")))
		}
	}

	// Master spec checks.
	masterContent, ok := contents[w.cfg.Spec]
	if ok {
		masterName := w.cfg.Spec

		// Feature index links.
		for name, fp := range features {
			found := false
			for _, m := range mdLinkRe.FindAllSubmatch(masterContent, -1) {
				target := string(m[2])
				targetBase := path.Base(target)
				if targetBase == path.Base(fp) || targetBase == name || strings.TrimSuffix(target, "/") == strings.TrimSuffix(fp, ".md") {
					found = true
					break
				}
			}
			if found {
				addResult(&results, &summary, "pass", "feature-index", fmt.Sprintf("%s linked from %s", fp, masterName))
			} else {
				addResult(&results, &summary, "warn", "feature-index", fmt.Sprintf("%s is not linked from %s", fp, masterName))
			}
		}
	}

	// Internal link validity across all spec files.
	for p, b := range contents {
		for _, m := range mdLinkRe.FindAllSubmatch(b, -1) {
			target := string(m[2])
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if strings.HasPrefix(target, "#") {
				continue
			}
			// Normalize target to a configured spec path.
			candidate := target
			if strings.HasSuffix(candidate, "/") {
				candidate = strings.TrimSuffix(candidate, "/") + ".md"
			}
			if !strings.HasSuffix(candidate, ".md") {
				candidate += ".md"
			}
			if !configured[candidate] {
				addResult(&results, &summary, "fail", "internal-links", fmt.Sprintf("%s links to missing spec %q", p, target))
			}
		}
	}

	// Duplicate feature names.
	seen := make(map[string]string)
	for p := range contents {
		if p == w.cfg.Spec {
			continue
		}
		base := strings.TrimSuffix(path.Base(p), ".md")
		if other, ok := seen[base]; ok {
			addResult(&results, &summary, "fail", "feature-names", fmt.Sprintf("feature name %q is used by both %s and %s", base, other, p))
		} else {
			seen[base] = p
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"results": results, "summary": summary})
		if summary.Fail > 0 {
			return 1
		}
		return 0
	}

	for _, r := range results {
		fmt.Fprintf(out, "[%s] %s: %s\n", strings.ToUpper(r.Level), r.Check, r.Message)
	}
	fmt.Fprintf(out, "\n%d passed, %d warnings, %d failures\n", summary.Pass, summary.Warn, summary.Fail)
	if summary.Fail > 0 {
		return 1
	}
	return 0
}
