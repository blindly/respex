package cli

import (
	"fmt"
	"io"
	"strconv"

	difflib "github.com/pmezard/go-difflib/difflib"

	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
	"github.com/blindly/respex/internal/ui"
)

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

	var fromLabel, toLabel string
	var oldC, newC []byte
	if len(args) == 2 && args[0] == "--refine" {
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
		oldC, newC = r.BeforeContent, r.AfterContent
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
			oldC, fromLabel = last.Content, fmt.Sprintf("v%d", last.ID)
			newC, err = spec.Read(w.specPath())
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
			oldC, fromLabel = va.Content, fmt.Sprintf("v%d", va.ID)
			newC, toLabel = vb.Content, fmt.Sprintf("v%d", vb.ID)
		default:
			return fail(errOut, fmt.Errorf("usage: respex diff [vA vB] | --refine <id|latest>"))
		}
	}

	text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(oldC)),
		B:        difflib.SplitLines(string(newC)),
		FromFile: fromLabel,
		ToFile:   toLabel,
		Context:  3,
	})
	if err != nil {
		return fail(errOut, err)
	}
	if text == "" {
		fmt.Fprintln(out, "no differences")
		return 0
	}
	if ui.IsTTY(out) {
		text = ui.ColorizeDiff(text)
	}
	fmt.Fprint(out, text)
	return 0
}
