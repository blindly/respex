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

	toml "github.com/pelletier/go-toml/v2"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
)

func runApply(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("apply", errOut)
	oneOff := fs.String("agent", "", `one-off adapter in TOML array form: --agent '["gemini", "-p", "{{prompt}}"]' (delivery mode is inherited from config)`)
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex apply [--agent tpl]", fs.Arg(0)))
	}

	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	content, err := spec.Read(w.specPath())
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
	if spec.Hash(content) != last.Hash {
		return fail(errOut, fmt.Errorf(
			"spec changed since last commit — review with `respex diff`, then `respex commit`"))
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, errors.New("another apply, refine, or restore is already running in this project"))
	}
	defer lock.Close()
	applied, err := st.IsApplied(last.ID)
	if err != nil {
		return fail(errOut, err)
	}
	if applied {
		fmt.Fprintf(out, "nothing to do (v%d already applied)\n", last.ID)
		return 0
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
	absSpec := w.absSpecPath()
	instr := agent.Expand(tmpl, "", absSpec)

	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	id, err := st.InsertApply(last.ID, tpl[0], time.Now())
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

	fmt.Fprintf(out, "applying v%d via %s; output: %s\n", last.ID, tpl[0], logRel)
	start := time.Now()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, instr, absSpec, f)
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
		return fail(errOut, fmt.Errorf("apply v%d failed (exit %d) — log: %s", last.ID, code, logRel))
	}
	fmt.Fprintf(out, "applied v%d via %s in %s — log: %s\n",
		last.ID, tpl[0], time.Since(start).Round(time.Second), logRel)
	if fi, err := os.Stat(filepath.Join(w.root, ".git")); err == nil && fi.IsDir() {
		fmt.Fprintln(out, "review the changes with `git diff`, then commit")
	}
	return 0
}
