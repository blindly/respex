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
	"github.com/blindly/respex/internal/config"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
	"github.com/blindly/respex/internal/ui"
)

const configTemplate = `# respex configuration
# spec = "SPEC.md"
# editor = ["code", "--wait"]
# agent_timeout = "1h"
# Add keys inside the tables below — do not redeclare [agent] or [prompts].

[agent]
# Required before refine/apply: choose one non-interactive agent command.
# Project config overrides this command when the same key is defined locally.
# Placeholders: {{prompt}} = instruction text, {{spec_path}} = spec file path.
# command = ["claude", "-p", "{{prompt}}"]
# command = ["devin", "--print", "{{prompt}}"]
# command = ["codex", "exec", "{{prompt}}"]
# command = ["gemini", "-p", "{{prompt}}"]
# command = ["opencode", "run", "{{prompt}}"]
# delivery = "argv"            # or "stdin": pipe the prompt, omit {{prompt}}
# env = []

[prompts]
# Optional overrides for the built-in prompt templates. {{spec_path}} works in
# all three; {{prompt}} is only substituted for draft (refine/apply pass none).
# draft  = "..."
# refine = "..."
# apply  = "..."
`

func runNew(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("new", errOut)
	noProgress := fs.Bool("no-progress", false, "disable the interactive progress indicator")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() > 1 {
		return fail(errOut, fmt.Errorf("usage: respex new [--no-progress] [description]"))
	}
	desc := ""
	if fs.NArg() == 1 {
		desc = fs.Arg(0)
	}
	if _, err := os.Stat(filepath.Join(".respex", "state.db")); err == nil {
		return fail(errOut, fmt.Errorf("this directory is already a respex project (.respex/state.db exists) — delete .respex/ and the spec file to re-initialize"))
	}
	cfg, err := loadConfig(".")
	if err != nil {
		return fail(errOut, err)
	}
	specPath := cfg.Spec
	if _, err := os.Stat(specPath); err == nil {
		return fail(errOut, fmt.Errorf("%s already exists", cfg.Spec))
	}
	if err := os.MkdirAll(".respex", 0o755); err != nil {
		return fail(errOut, err)
	}
	if _, err := os.Stat(".respex/config.toml"); err != nil {
		if err := os.WriteFile(".respex/config.toml", []byte(configTemplate), 0o644); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintln(out, "created .respex/config.toml")
	}
	st, err := state.Open(filepath.Join(".respex", "state.db"))
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()

	if err := ensureGitignore(); err != nil {
		return fail(errOut, err)
	}

	switch {
	case desc != "" && len(cfg.Agent.Command) > 0:
		if code := draftSpec(cfg, desc, cfg.Spec, *noProgress, out, errOut); code != 0 {
			return code
		}
		fmt.Fprintln(out, "edit the spec, then run `respex commit` before `respex apply`")
	default:
		if err := os.WriteFile(cfg.Spec, []byte(spec.Skeleton), 0o644); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintf(out, "created %s (skeleton)\n", cfg.Spec)
		if desc != "" {
			fmt.Fprintln(out, "no agent configured — set [agent] command in .respex/config.toml, then run `respex refine`")
		} else {
			fmt.Fprintln(out, "edit the spec, then run `respex commit` before `respex apply`")
		}
	}
	return 0
}

// draftSpec runs the agent to write the spec file from a description.
func draftSpec(cfg config.Config, desc, specRel string, noProgress bool, out, errOut io.Writer) int {
	delivery := cfg.Agent.Delivery
	if delivery == "" {
		delivery = agent.DeliveryArgv
	}
	a := agent.Adapter{Command: cfg.Agent.Command, Delivery: delivery, Env: cfg.Agent.Env, Dir: "."}
	logsDir := filepath.Join(".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-draft-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	logPath := f.Name()
	abs, err := filepath.Abs(specRel)
	if err != nil {
		return fail(errOut, err)
	}
	tmpl := agent.PromptDraft
	if cfg.Prompts.Draft != "" {
		tmpl = cfg.Prompts.Draft
	}
	label := fmt.Sprintf("drafting %s via %s", specRel, cfg.Agent.Command[0])
	progressEnabled := ui.IsTTY(out) && !noProgress && os.Getenv("NO_COLOR") == ""
	if !progressEnabled {
		fmt.Fprintf(out, "%s; output: %s\n", label, logPath)
	}
	progress := ui.StartProgress(out, label, progressEnabled)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, agent.Expand(tmpl, desc, abs), abs, f)
	progress.Stop()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fail(errOut, fmt.Errorf("draft timed out after %s — log: %s", cfg.AgentTimeout, logPath))
		}
		if errors.Is(err, context.Canceled) {
			return fail(errOut, fmt.Errorf("draft interrupted — log: %s", logPath))
		}
		return fail(errOut, err)
	}
	if code != 0 {
		return fail(errOut, fmt.Errorf("draft failed (exit %d) — log: %s", code, logPath))
	}
	if _, err := os.Stat(specRel); err != nil {
		return fail(errOut, fmt.Errorf("agent did not create %s — log: %s", specRel, logPath))
	}
	fmt.Fprintf(out, "drafted %s via %s — log: %s\n", specRel, cfg.Agent.Command[0], logPath)
	return 0
}

// ensureGitignore appends .respex/ to .gitignore when this is a git repo.
func ensureGitignore() error {
	if fi, err := os.Stat(".git"); err != nil || !fi.IsDir() {
		return nil
	}
	existing, err := os.ReadFile(".gitignore")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == ".respex/" {
			return nil
		}
	}
	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	// A leading blank line is needed only to terminate a pre-existing entry
	// whose last line lacks a newline; a fresh file starts clean.
	prefix := ""
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + ".respex/\n")
	return err
}
