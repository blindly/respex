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

func runRefine(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("refine", errOut)
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex refine [--no-progress]", fs.Arg(0)))
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
		return fail(errOut, errors.New("another apply, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	before, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	beforeHash := spec.Hash(before)
	a, name, err := w.adapter()
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	last, err := st.LatestVersion()
	if err != nil {
		return fail(errOut, err)
	}
	if last != nil && last.Hash != beforeHash {
		fmt.Fprintln(out, "refining a modified working spec; current content will be saved in refinement history")
	}

	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-refine-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	logPath := f.Name()
	logRel := filepath.Join(".respex", "logs", filepath.Base(logPath))
	started := time.Now()
	record := func(outcome, afterHash string, after []byte) error {
		if after == nil {
			after = []byte{}
		}
		_, err := st.InsertRefine(name, outcome, beforeHash, afterHash, before, after, logRel, started, time.Now())
		return err
	}

	tmpl := agent.PromptRefine
	if w.cfg.Prompts.Refine != "" {
		tmpl = w.cfg.Prompts.Refine
	}
	absSpec := w.absSpecPath()
	label := fmt.Sprintf("refining via %s", name)
	progressEnabled := ui.IsTTY(out) && !*noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logRel)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, agent.Expand(tmpl, "", absSpec), absSpec, f)
	progress.Stop()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if rerr := record("timed_out", beforeHash, before); rerr != nil {
				return fail(errOut, rerr)
			}
			return fail(errOut, fmt.Errorf("refine timed out after %s — log: %s", w.cfg.AgentTimeout, logPath))
		}
		if errors.Is(err, context.Canceled) {
			if rerr := record("interrupted", beforeHash, before); rerr != nil {
				return fail(errOut, rerr)
			}
			fmt.Fprintf(out, "refine interrupted — log: %s\n", logPath)
			return 1
		}
		if rerr := record("failed", beforeHash, before); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, err)
	}
	if code != 0 {
		if rerr := record("failed", beforeHash, before); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("refine failed (exit %d) — log: %s", code, logPath))
	}
	after, err := spec.Read(w.specPath())
	if err != nil {
		if rerr := record("failed", "", nil); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("agent removed the spec — log: %s: %w", logPath, err))
	}
	afterHash := spec.Hash(after)
	if afterHash == beforeHash {
		if err := record("unchanged", afterHash, after); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintln(out, "spec unchanged")
		return 0
	}
	if err := record("updated", afterHash, after); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "spec updated via %s (%s… → %s…) — log: %s\n",
		name, beforeHash[:8], afterHash[:8], logPath)
	return 0
}
