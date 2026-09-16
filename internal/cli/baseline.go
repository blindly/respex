package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/ui"
)

func runBaseline(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("baseline", errOut)
	intent := fs.String("intent", "", "additional project intent for the agent")
	merge := fs.Bool("merge", false, "merge repository findings into an existing spec")
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex baseline [--intent text] [--merge] [--no-progress]", fs.Arg(0)))
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
		return fail(errOut, errors.New("another apply, baseline, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	before, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	if spec.Hash(before) != spec.Hash([]byte(spec.Skeleton)) && !*merge {
		return fail(errOut, errors.New("the spec contains meaningful content — review it and rerun with `respex baseline --merge`"))
	}
	a, name, err := w.adapter()
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-baseline-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	logPath := f.Name()
	logRel := filepath.Join(".respex", "logs", filepath.Base(logPath))
	started := time.Now()
	record := func(outcome string, after []byte) error {
		if after == nil {
			after = []byte{}
		}
		afterHash := ""
		if len(after) > 0 {
			afterHash = spec.Hash(after)
		}
		_, err := st.InsertBaseline(name, outcome, spec.Hash(before), afterHash, before, after, logRel, started, time.Now())
		return err
	}
	tmpl := agent.PromptBaseline
	if w.cfg.Prompts.Baseline != "" {
		tmpl = w.cfg.Prompts.Baseline
	}
	if *merge {
		tmpl += "\n\nMerge findings into the existing specification. Preserve established user intent and explicitly documented requirements unless they directly contradict observed behavior; record conflicts in Open Questions."
	}
	absSpec := w.absSpecPath()
	label := fmt.Sprintf("baselining repository via %s", name)
	progressEnabled := ui.IsTTY(out) && !*noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logRel)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, agent.Expand(tmpl, *intent, absSpec), absSpec, f)
	progress.Stop()
	if err != nil {
		outcome := "failed"
		if errors.Is(err, context.DeadlineExceeded) {
			outcome = "timed_out"
		} else if errors.Is(err, context.Canceled) {
			outcome = "interrupted"
		}
		if rerr := record(outcome, before); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("baseline %s — log: %s", outcome, logPath))
	}
	if code != 0 {
		if err := record("failed", before); err != nil {
			return fail(errOut, err)
		}
		return fail(errOut, fmt.Errorf("baseline failed (exit %d) — log: %s", code, logPath))
	}
	after, err := spec.Read(w.specPath())
	if err != nil {
		if rerr := record("failed", nil); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("agent removed the spec — log: %s: %w", logPath, err))
	}
	outcome := "generated"
	if spec.Hash(before) == spec.Hash(after) {
		outcome = "unchanged"
	} else if *merge {
		outcome = "merged"
	}
	if err := record(outcome, after); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "baseline %s via %s (%s… → %s…) — log: %s\n", outcome, name, spec.Hash(before)[:8], spec.Hash(after)[:8], logRel)
	fmt.Fprintln(out, "review with `respex diff --baseline latest`, then commit")
	return 0
}
