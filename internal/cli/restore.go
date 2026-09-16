package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/blindly/respex/internal/spec"
)

func runRestore(args []string, out, errOut io.Writer) int {
	if len(args) != 3 || args[0] != "--refine" || args[2] != "--before" {
		return fail(errOut, fmt.Errorf("usage: respex restore --refine <id|latest> --before"))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, fmt.Errorf("another apply, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	var refinementID int64
	if args[1] == "latest" {
		r, err := st.LatestRefine()
		if err != nil {
			return fail(errOut, err)
		}
		if r == nil {
			return fail(errOut, fmt.Errorf("no refinements yet"))
		}
		refinementID = r.ID
	} else {
		refinementID, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fail(errOut, fmt.Errorf("usage: respex restore --refine <id|latest> --before"))
		}
	}
	r, err := st.GetRefine(refinementID)
	if err != nil {
		return fail(errOut, err)
	}
	if len(r.BeforeContent) == 0 {
		return fail(errOut, fmt.Errorf("refinement #%d has no saved before-state", r.ID))
	}
	current, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	started := time.Now()
	if err := os.WriteFile(w.specPath(), r.BeforeContent, 0o644); err != nil {
		return fail(errOut, fmt.Errorf("restore spec: %w", err))
	}
	restoreID, err := st.InsertRefine("respex", "restored", spec.Hash(current), spec.Hash(r.BeforeContent), current, r.BeforeContent, "", started, time.Now())
	if err != nil {
		if rollbackErr := os.WriteFile(w.specPath(), current, 0o644); rollbackErr != nil {
			return fail(errOut, fmt.Errorf("record restore: %v; rollback spec: %w", err, rollbackErr))
		}
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "restored spec to before refinement #%d; previous content saved as refinement #%d\n", r.ID, restoreID)
	return 0
}
