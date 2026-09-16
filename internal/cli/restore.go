package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/blindly/respex/internal/spec"
)

func runRestore(args []string, out, errOut io.Writer) int {
	if len(args) != 3 || (args[0] != "--refine" && args[0] != "--baseline") || args[2] != "--before" {
		return fail(errOut, fmt.Errorf("usage: respex restore <--refine|--baseline> <id|latest> --before"))
	}
	kind := args[0][2:]
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, fmt.Errorf("another apply, baseline, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	var id int64
	var before []byte
	if args[1] == "latest" {
		if kind == "refine" {
			r, err := st.LatestRefine()
			if err != nil {
				return fail(errOut, err)
			}
			if r == nil {
				return fail(errOut, fmt.Errorf("no refinements yet"))
			}
			id, before = r.ID, r.BeforeContent
		} else {
			b, err := st.LatestBaseline()
			if err != nil {
				return fail(errOut, err)
			}
			if b == nil {
				return fail(errOut, fmt.Errorf("no baselines yet"))
			}
			id, before = b.ID, b.BeforeContent
		}
	} else {
		id, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fail(errOut, fmt.Errorf("usage: respex restore <--refine|--baseline> <id|latest> --before"))
		}
		if kind == "refine" {
			r, err := st.GetRefine(id)
			if err != nil {
				return fail(errOut, err)
			}
			before = r.BeforeContent
		} else {
			b, err := st.GetBaseline(id)
			if err != nil {
				return fail(errOut, err)
			}
			before = b.BeforeContent
		}
	}
	if len(before) == 0 {
		return fail(errOut, fmt.Errorf("%s #%d has no saved before-state", kind, id))
	}
	current, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	candidate, cleanup, err := createSpecCandidate(w.root, before)
	if err != nil {
		return fail(errOut, err)
	}
	defer cleanup()
	started := time.Now()
	if _, err := installSpecCandidate(w.specPath(), candidate, current); err != nil {
		return fail(errOut, fmt.Errorf("restore spec: %w", err))
	}
	restoreID, err := st.InsertRefine("respex", "restored", spec.Hash(current), spec.Hash(before), current, before, "", "", started, time.Now())
	if err != nil {
		rollback, cleanupRollback, createErr := createSpecCandidate(w.root, current)
		if createErr == nil {
			_, createErr = installSpecCandidate(w.specPath(), rollback, before)
			cleanupRollback()
		}
		if createErr != nil {
			return fail(errOut, fmt.Errorf("record restore: %v; rollback spec: %w", err, createErr))
		}
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "restored spec to before %s #%d; previous content saved as refinement #%d\n", kind, id, restoreID)
	return 0
}
