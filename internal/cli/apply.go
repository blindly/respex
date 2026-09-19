package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/ui"
)

func runApply(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("apply", errOut)
	oneOff := fs.String("agent", "", `one-off adapter in TOML array form: --agent '["gemini", "-p", "{{prompt}}"]' (delivery mode is inherited from config)`)
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() > 1 {
		return fail(errOut, errors.New("usage: respex apply [--agent tpl] [--no-progress] [feature]"))
	}
	featureName := ""
	if fs.NArg() == 1 {
		featureName = fs.Arg(0)
	}

	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	if featureName != "" && len(w.cfg.SpecFiles) == 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex apply [--agent tpl] [--no-progress]", featureName))
	}
	featurePath, err := w.resolveFeature(featureName)
	if err != nil {
		return fail(errOut, err)
	}
	workingHash, err := w.workingSpecHash()
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
	if last == nil {
		return fail(errOut, fmt.Errorf("no committed versions yet — run `respex commit`"))
	}
	if workingHash != last.Hash {
		return fail(errOut, fmt.Errorf(
			"spec changed since last commit — review with `respex diff`, then `respex commit`"))
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, errors.New("another apply, baseline, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	featureLabel := ""
	if featurePath != "" {
		featureLabel = path.Base(featurePath)
	}
	verifyConfigured := len(w.cfg.Verify.Commands) > 0 || w.cfg.Verify.Audit
	if featurePath == "" {
		conformed, reason, err := st.Conformed(last.ID, verifyConfigured)
		if err != nil {
			return fail(errOut, err)
		}
		if conformed {
			fmt.Fprintf(out, "nothing to do (v%d already applied)\n", last.ID)
			return 0
		}
		if reason != "no successful apply" {
			fmt.Fprintf(out, "v%d was applied but %s — re-applying\n", last.ID, reason)
		}
	} else {
		applied, err := st.IsFeatureApplied(last.ID, featurePath)
		if err != nil {
			return fail(errOut, err)
		}
		if applied {
			fmt.Fprintf(out, "nothing to do (feature %s of v%d already applied)\n", featureLabel, last.ID)
			return 0
		}
	}
	if unfinished, err := st.HasUnfinishedApply(); err != nil {
		return fail(errOut, err)
	} else if unfinished {
		if err := st.MarkUnfinishedAppliesStale(); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintln(out, "warning: a previous apply did not finish; re-running")
	}

	tpl := w.cfg.Agent.Command
	if *oneOff != "" {
		var parsed struct {
			Command []string `toml:"command"`
		}
		if err := toml.Unmarshal([]byte("command = "+*oneOff), &parsed); err != nil || len(parsed.Command) == 0 {
			return fail(errOut, fmt.Errorf("invalid --agent template %q", *oneOff))
		}
		tpl = parsed.Command
	}
	if len(tpl) == 0 {
		return fail(errOut, errors.New("no agent configured — set [agent] command in .respex/config.toml"))
	}
	delivery := w.cfg.Agent.Delivery
	if delivery == "" {
		delivery = agent.DeliveryArgv
	}
	a := agent.Adapter{Command: tpl, Delivery: delivery, Env: w.cfg.Agent.Env, Dir: w.root}

	tmpl := agent.PromptApply
	if w.cfg.Prompts.Apply != "" {
		tmpl = w.cfg.Prompts.Apply
	}
	master, err := spec.CleanSpecPath(w.cfg.Spec)
	if err != nil {
		return fail(errOut, err)
	}
	absSpec, cleanupSpec, err := materializeSnapshot(w.root, last.ID, versionFiles(last.Content, master))
	if err != nil {
		return fail(errOut, err)
	}
	defer cleanupSpec()
	instr := agent.Expand(tmpl, "", absSpec)
	featureSnapPath := ""
	if featurePath != "" {
		featureSnapPath = filepath.Join(absSpec, featurePath)
		masterSnapPath := filepath.Join(absSpec, master)
		instr = agent.Expand(agent.PromptApplyFeature, "", featureSnapPath, masterSnapPath)
	}

	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	id, err := st.InsertApply(last.ID, tpl[0], featurePath, time.Now())
	if err != nil {
		return fail(errOut, err)
	}
	logRel := filepath.Join(".respex", "logs", fmt.Sprintf("%d-apply.log", id))
	if err := st.SetApplyLogPath(id, logRel); err != nil {
		if ferr := st.FinishApply(id, -1, time.Now()); ferr != nil {
			return fail(errOut, ferr)
		}
		return fail(errOut, err)
	}
	f, err := os.Create(filepath.Join(w.root, logRel))
	if err != nil {
		if ferr := st.FinishApply(id, -1, time.Now()); ferr != nil {
			return fail(errOut, ferr)
		}
		return fail(errOut, err)
	}
	defer f.Close()

	label := fmt.Sprintf("applying v%d via %s", last.ID, tpl[0])
	if featurePath != "" {
		label = fmt.Sprintf("applying feature %s of v%d via %s", featureLabel, last.ID, tpl[0])
	}
	progressEnabled := ui.IsTTY(out) && !*noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logRel)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	start := time.Now()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, instr, absSpec, f)
	progress.Stop()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if ferr := st.FinishApplyWithOutcome(id, -1, "timed_out", time.Now()); ferr != nil {
				return fail(errOut, ferr)
			}
			return fail(errOut, fmt.Errorf("apply v%d timed out after %s — log: %s", last.ID, w.cfg.AgentTimeout, logRel))
		}
		if errors.Is(err, context.Canceled) {
			if ferr := st.FinishApplyWithOutcome(id, -1, "interrupted", time.Now()); ferr != nil {
				return fail(errOut, ferr)
			}
			fmt.Fprintf(out, "apply interrupted — log: %s\n", logRel)
			return 1
		}
		// Non-interrupt execution error (e.g. agent binary missing): stamp the
		// row with exit_code -1 so no stale "did not finish" warning persists.
		if ferr := st.FinishApply(id, -1, time.Now()); ferr != nil {
			return fail(errOut, ferr)
		}
		return fail(errOut, err)
	}
	if err := st.FinishApply(id, code, time.Now()); err != nil {
		return fail(errOut, err)
	}
	if code != 0 {
		if featurePath != "" {
			return fail(errOut, fmt.Errorf("apply feature %s of v%d failed (exit %d) — log: %s", featureLabel, last.ID, code, logRel))
		}
		return fail(errOut, fmt.Errorf("apply v%d failed (exit %d) — log: %s", last.ID, code, logRel))
	}
	if featurePath != "" {
		fmt.Fprintf(out, "applied feature %s of v%d via %s in %s — log: %s\n",
			featureLabel, last.ID, tpl[0], time.Since(start).Round(time.Second), logRel)
	} else {
		fmt.Fprintf(out, "applied v%d via %s in %s — log: %s\n",
			last.ID, tpl[0], time.Since(start).Round(time.Second), logRel)
	}
	if verifyConfigured && featurePath == "" {
		run, code := runVerification(w, st, last, out, errOut)
		if code != 0 {
			return fail(errOut, fmt.Errorf("verification after apply failed — fix the working tree and run `respex verify`; log: %s", run.LogPath))
		}
	}
	if fi, err := os.Stat(filepath.Join(w.root, ".git")); err == nil && fi.IsDir() {
		fmt.Fprintln(out, "review the changes with `git diff`, then commit")
	}
	return 0
}
