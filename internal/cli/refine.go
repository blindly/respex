package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/ui"
)

func refineFingerprint(specHash, prompt string, adapter agent.Adapter) string {
	parts := []string{specHash, prompt, adapter.Delivery, strings.Join(adapter.Command, "\x00"), strings.Join(adapter.Env, "\x00")}
	return spec.Hash([]byte(strings.Join(parts, "\x01")))
}

func runRefine(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("refine", errOut)
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	force := fs.Bool("force", false, "run even when refinement is likely to be a no-op")
	withNotes := fs.Bool("notes", false, "also read project notes as context")

	// Allow the optional feature name to appear before or after boolean flags.
	featureName := ""
	var flagArgs []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
		} else if featureName == "" {
			featureName = a
		} else {
			return fail(errOut, errors.New("usage: respex refine [--force] [--no-progress] [feature]"))
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, errors.New("usage: respex refine [--force] [--no-progress] [feature]"))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	if featureName != "" && len(w.cfg.SpecFiles) == 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex refine [--force] [--no-progress]", featureName))
	}
	targetPath, err := w.resolveFeature(featureName)
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

	livePath := w.specPath()
	if targetPath != "" {
		livePath = filepath.Join(w.root, targetPath)
	}
	before, err := spec.Read(livePath)
	if err != nil {
		return fail(errOut, err)
	}
	beforeHash := spec.Hash(before)
	if targetPath == "" && !*force && beforeHash == spec.Hash([]byte(spec.Skeleton)) {
		return fail(errOut, errors.New("the spec is still the generated skeleton — run `respex baseline` for an existing repository or `respex edit` first; use `respex refine --force`"))
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
	candidatePath, cleanupCandidate, err := createSpecCandidate(w.root, before)
	if err != nil {
		return fail(errOut, err)
	}
	defer cleanupCandidate()
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
	inputFingerprint := ""
	record := func(outcome, afterHash string, after []byte) error {
		if after == nil {
			after = []byte{}
		}
		_, err := st.InsertRefine(name, outcome, beforeHash, afterHash, before, after, targetPath, inputFingerprint, logRel, started, time.Now())
		return err
	}

	tmpl := agent.PromptRefine
	if targetPath != "" {
		tmpl = agent.PromptRefineFeature
	}
	if w.cfg.Prompts.Refine != "" {
		tmpl = w.cfg.Prompts.Refine
	}
	inputFingerprint = refineFingerprint(beforeHash, tmpl+targetPath, a)
	latestRefine, err := st.LatestRefineByTarget(targetPath)
	if err != nil {
		return fail(errOut, err)
	}
	if !*force && latestRefine != nil && latestRefine.Outcome == "unchanged" && latestRefine.AfterHash == beforeHash && latestRefine.InputFingerprint == inputFingerprint {
		f.Close()
		_ = os.Remove(logPath)
		return fail(errOut, errors.New("this exact spec, prompt, and agent configuration was last refined unchanged — edit the spec or use `respex refine --force`"))
	}
	absSpec := candidatePath
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
	instr := agent.Expand(tmpl, "", absSpec)
	if targetPath != "" {
		instr += fmt.Sprintf("\n\nThe master spec at %s provides project-wide context. Read it, but do not modify it. Rewrite only the feature spec at %s.", w.absSpecPath(), absSpec)
	}
	if *withNotes {
		notesPath := w.notesPath()
		if _, err := os.Stat(notesPath); err == nil {
			instr += fmt.Sprintf("\n\nAlso read the project notes at %s for additional context and ideas. Incorporate anything that is ready into the spec, but leave unresolved ideas in the notes file.", notesPath)
		}
	}
	code, err := a.Execute(ctx, instr, absSpec, f)
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
	after, err := installSpecCandidate(livePath, candidatePath, before)
	if err != nil {
		candidate, _ := os.ReadFile(candidatePath)
		if rerr := record("failed", "", candidate); rerr != nil {
			return fail(errOut, rerr)
		}
		return fail(errOut, fmt.Errorf("install refined spec — log: %s: %w", logPath, err))
	}
	warnMissingSections(out, after)
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
