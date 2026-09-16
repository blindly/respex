package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
)

func runLog(args []string, out, errOut io.Writer) int {
	jsonOutput := len(args) == 1 && args[0] == "--json"
	if len(args) != 0 && !jsonOutput {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex log [--json]", args[0]))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	versions, err := st.ListVersions()
	if err != nil {
		return fail(errOut, err)
	}
	applies, err := st.ListApplies()
	if err != nil {
		return fail(errOut, err)
	}
	refines, err := st.ListRefines()
	if err != nil {
		return fail(errOut, err)
	}
	baselines, err := st.ListBaselines()
	if err != nil {
		return fail(errOut, err)
	}
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(map[string]any{"versions": versions, "applies": applies, "baselines": baselines, "refinements": refines}); err != nil {
			return fail(errOut, err)
		}
		return 0
	}

	fmt.Fprintln(out, "versions:")
	if len(versions) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, v := range versions {
		if v.Message == "" {
			fmt.Fprintf(out, "  v%d  %s  %s\n", v.ID, v.Hash[:8], v.CommittedAt.Format(time.RFC3339))
			continue
		}
		fmt.Fprintf(out, "  v%d  %s  %s  %s\n", v.ID, v.Hash[:8],
			v.CommittedAt.Format(time.RFC3339), strings.ReplaceAll(v.Message, "\n", " "))
	}
	fmt.Fprintln(out, "applies:")
	if len(applies) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, a := range applies {
		fmt.Fprintf(out, "  #%d  v%d  %s  %s  %s  %s\n", a.ID, a.VersionID, a.Agent,
			a.Outcome, a.StartedAt.Format(time.RFC3339), a.LogPath)
	}
	fmt.Fprintln(out, "baselines:")
	if len(baselines) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, b := range baselines {
		fmt.Fprintf(out, "  #%d  %s  %s  %s  %s\n", b.ID, b.Agent, b.Outcome,
			b.StartedAt.Format(time.RFC3339), b.LogPath)
	}
	fmt.Fprintln(out, "refinements:")
	if len(refines) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, r := range refines {
		fmt.Fprintf(out, "  #%d  %s  %s  %s  %s\n", r.ID, r.Agent, r.Outcome,
			r.StartedAt.Format(time.RFC3339), r.LogPath)
	}
	return 0
}

func runStatus(args []string, out, errOut io.Writer) int {
	jsonOutput := len(args) == 1 && args[0] == "--json"
	if len(args) != 0 && !jsonOutput {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex status [--json]", args[0]))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()

	display := out
	if jsonOutput {
		display = io.Discard
	}
	absSpec := w.absSpecPath()
	fmt.Fprintf(display, "spec:        %s\n", absSpec)
	lastV, err := st.LatestVersion()
	if err != nil {
		return fail(errOut, err)
	}
	content, readErr := os.ReadFile(absSpec)
	dirty := "no"
	last := "none"
	applied := "-"
	if readErr != nil {
		dirty = "missing"
		if !errors.Is(readErr, fs.ErrNotExist) {
			dirty = "unreadable"
		}
	}
	if lastV != nil {
		last = fmt.Sprintf("v%d %s… %s", lastV.ID, lastV.Hash[:8], lastV.CommittedAt.Format(time.RFC3339))
		ok, err := st.IsApplied(lastV.ID)
		if err != nil {
			return fail(errOut, err)
		}
		applied = "no"
		if ok {
			applied = "yes"
		}
		if readErr == nil && spec.Hash(content) != lastV.Hash {
			dirty = "yes"
		}
	}
	fmt.Fprintf(display, "dirty:       %s\n", dirty)
	fmt.Fprintf(display, "last commit: %s\n", last)
	fmt.Fprintf(display, "applied:     %s\n", applied)
	applyState := "idle"
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, err)
	}
	if !locked {
		unfinished, err := st.HasUnfinishedApply()
		if err != nil {
			return fail(errOut, err)
		}
		if unfinished {
			applyState = "running"
		} else {
			applyState = "baseline, refine, restore, or edit running"
		}
	} else {
		lock.Close()
		if unfinished, err := st.HasUnfinishedApply(); err != nil {
			return fail(errOut, err)
		} else if unfinished {
			applyState = "previous run ended unexpectedly"
		}
	}
	fmt.Fprintf(display, "apply:       %s\n", applyState)
	refines, err := st.ListRefines()
	if err != nil {
		return fail(errOut, err)
	}
	refined := "never"
	if len(refines) > 0 {
		refined = fmt.Sprintf("%d times; last %s at %s", len(refines), refines[0].Outcome, refines[0].StartedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(display, "refined:     %s\n", refined)
	baselines, err := st.ListBaselines()
	if err != nil {
		return fail(errOut, err)
	}
	baselineState := "never"
	if len(baselines) > 0 {
		baselineState = fmt.Sprintf("%d times; last %s at %s", len(baselines), baselines[0].Outcome, baselines[0].StartedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(display, "baselined:   %s\n", baselineState)
	refineState := "ready"
	if readErr == nil {
		workingHash := spec.Hash(content)
		if workingHash == spec.Hash([]byte(spec.Skeleton)) {
			refineState = "untouched skeleton; baseline the repository or edit the spec"
		} else if len(refines) > 0 && refines[0].Outcome == "unchanged" && refines[0].AfterHash == workingHash {
			if adapter, _, adapterErr := w.adapter(); adapterErr == nil {
				tmpl := agent.PromptRefine
				if w.cfg.Prompts.Refine != "" {
					tmpl = w.cfg.Prompts.Refine
				}
				if refines[0].InputFingerprint == refineFingerprint(workingHash, tmpl, adapter) {
					refineState = "same input was last unchanged; edit or use --force"
				}
			}
		}
	}
	fmt.Fprintf(display, "refine:      %s\n", refineState)
	agentLine := "(not configured)"
	if len(w.cfg.Agent.Command) > 0 {
		agentLine = w.cfg.Agent.Command[0] + " (configured)"
	}
	fmt.Fprintf(display, "agent:       %s\n", agentLine)
	if jsonOutput {
		payload := map[string]any{"spec": absSpec, "dirty": dirty, "last_commit": last, "applied": applied, "apply_state": applyState, "refined": refined, "baselined": baselineState, "refine_state": refineState, "agent": agentLine}
		if err := json.NewEncoder(out).Encode(payload); err != nil {
			return fail(errOut, err)
		}
	}
	return 0
}
