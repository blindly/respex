package cli

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	difflib "github.com/pmezard/go-difflib/difflib"

	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
	"github.com/blindly/respex/internal/ui"
)

// emitDiffs prints a unified diff per bundle path. A single-file pair keeps
// the plain from/to labels; multi-file diffs are labeled <label>:<path>.
func emitDiffs(out, errOut io.Writer, oldFiles, newFiles []spec.File, fromLabel, toLabel string) int {
	oldByPath := make(map[string][]byte, len(oldFiles))
	for _, f := range oldFiles {
		oldByPath[f.Path] = f.Content
	}
	newByPath := make(map[string][]byte, len(newFiles))
	for _, f := range newFiles {
		newByPath[f.Path] = f.Content
	}
	var order []string
	seen := map[string]bool{}
	for _, f := range oldFiles {
		if !seen[f.Path] {
			seen[f.Path] = true
			order = append(order, f.Path)
		}
	}
	for _, f := range newFiles {
		if !seen[f.Path] {
			seen[f.Path] = true
			order = append(order, f.Path)
		}
	}
	single := len(order) == 1
	var text bytes.Buffer
	for _, p := range order {
		oldC, hasOld := oldByPath[p]
		newC, hasNew := newByPath[p]
		if hasOld && hasNew && bytes.Equal(oldC, newC) {
			continue
		}
		from, to := fromLabel, toLabel
		if !single {
			from, to = fromLabel+":"+p, toLabel+":"+p
		}
		chunk, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(oldC)),
			B:        difflib.SplitLines(string(newC)),
			FromFile: from,
			ToFile:   to,
			Context:  3,
		})
		if err != nil {
			return fail(errOut, err)
		}
		text.WriteString(chunk)
	}
	if text.Len() == 0 {
		fmt.Fprintln(out, "no differences")
		return 0
	}
	s := text.String()
	if ui.IsTTY(out) {
		s = ui.ColorizeDiff(s)
	}
	fmt.Fprint(out, s)
	return 0
}

func runDiff(args []string, out, errOut io.Writer) int {
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	master, err := spec.CleanSpecPath(w.cfg.Spec)
	if err != nil {
		return fail(errOut, err)
	}

	var fromLabel, toLabel string
	var oldFiles, newFiles []spec.File
	if len(args) == 2 && args[0] == "--baseline" {
		var b *state.Baseline
		if args[1] == "latest" {
			b, err = st.LatestBaseline()
			if err == nil && b == nil {
				return fail(errOut, fmt.Errorf("no baselines yet"))
			}
		} else {
			id, parseErr := strconv.Atoi(args[1])
			if parseErr != nil {
				return fail(errOut, fmt.Errorf("usage: respex diff --baseline <id|latest>"))
			}
			b, err = st.GetBaseline(int64(id))
		}
		if err != nil {
			return fail(errOut, err)
		}
		fromLabel, toLabel = fmt.Sprintf("baseline-%d-before", b.ID), fmt.Sprintf("baseline-%d-after", b.ID)
		if len(b.Proposal) > 0 {
			p, decErr := decodeProposal(b)
			if decErr != nil {
				return fail(errOut, decErr)
			}
			oldFiles, newFiles = p.Before, p.After
		} else {
			oldFiles = []spec.File{{Path: master, Content: b.BeforeContent}}
			newFiles = []spec.File{{Path: master, Content: b.AfterContent}}
		}
	} else if len(args) == 2 && args[0] == "--refine" {
		var r *state.Refine
		if args[1] == "latest" {
			r, err = st.LatestRefine()
			if err == nil && r == nil {
				return fail(errOut, fmt.Errorf("no refinements yet"))
			}
		} else {
			id, parseErr := strconv.Atoi(args[1])
			if parseErr != nil {
				return fail(errOut, fmt.Errorf("usage: respex diff --refine <id|latest>"))
			}
			r, err = st.GetRefine(int64(id))
		}
		if err != nil {
			return fail(errOut, err)
		}
		oldFiles = []spec.File{{Path: master, Content: r.BeforeContent}}
		newFiles = []spec.File{{Path: master, Content: r.AfterContent}}
		fromLabel, toLabel = fmt.Sprintf("refine-%d-before", r.ID), fmt.Sprintf("refine-%d-after", r.ID)
	} else {
		switch len(args) {
		case 0:
			last, err := st.LatestVersion()
			if err != nil {
				return fail(errOut, err)
			}
			if last == nil {
				return fail(errOut, fmt.Errorf("no committed versions yet"))
			}
			oldFiles = versionFiles(last.Content, master)
			fromLabel = fmt.Sprintf("v%d", last.ID)
			newFiles, err = w.readSpecFiles()
			if err != nil {
				return fail(errOut, err)
			}
			toLabel = "working"
		case 2:
			a, errA := strconv.Atoi(args[0])
			b, errB := strconv.Atoi(args[1])
			if errA != nil || errB != nil {
				return fail(errOut, fmt.Errorf("usage: respex diff [vA vB] — version ids are integers"))
			}
			va, err := st.GetVersion(int64(a))
			if err != nil {
				return fail(errOut, err)
			}
			vb, err := st.GetVersion(int64(b))
			if err != nil {
				return fail(errOut, err)
			}
			oldFiles = versionFiles(va.Content, master)
			fromLabel = fmt.Sprintf("v%d", va.ID)
			newFiles = versionFiles(vb.Content, master)
			toLabel = fmt.Sprintf("v%d", vb.ID)
		default:
			return fail(errOut, fmt.Errorf("usage: respex diff [vA vB] | --refine <id|latest> | --baseline <id|latest>"))
		}
	}

	return emitDiffs(out, errOut, oldFiles, newFiles, fromLabel, toLabel)
}
