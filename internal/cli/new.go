package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"respex/internal/agent"
	"respex/internal/config"
	"respex/internal/spec"
	"respex/internal/state"
)

const configTemplate = `# respex configuration
# spec = "SPEC.md"
# Add keys inside the tables below — do not redeclare [agent] or [prompts].

[agent]
# Required before refine/apply: the agent CLI, as an argv array.
# Placeholders: {{prompt}} = instruction text, {{spec_path}} = spec file path.
# command = ["claude", "-p", "{{prompt}}"]
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
	if len(args) > 1 {
		return fail(errOut, fmt.Errorf("usage: respex new [description]"))
	}
	desc := ""
	if len(args) == 1 {
		desc = args[0]
	}
	if _, err := os.Stat(filepath.Join(".respex", "state.db")); err == nil {
		return fail(errOut, fmt.Errorf("this directory is already a respex project (.respex/state.db exists) — delete .respex/state.db to re-initialize"))
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
		if code := draftSpec(cfg, desc, cfg.Spec, out, errOut); code != 0 {
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
func draftSpec(cfg config.Config, desc, specRel string, out, errOut io.Writer) int {
	delivery := cfg.Agent.Delivery
	if delivery == "" {
		delivery = agent.DeliveryArgv
	}
	a := agent.Adapter{Command: cfg.Agent.Command, Delivery: delivery, Env: cfg.Agent.Env, Dir: "."}
	logsDir := filepath.Join(".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	logPath := filepath.Join(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-draft.log")
	f, err := os.Create(logPath)
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	abs, err := filepath.Abs(specRel)
	if err != nil {
		return fail(errOut, err)
	}
	tmpl := agent.PromptDraft
	if cfg.Prompts.Draft != "" {
		tmpl = cfg.Prompts.Draft
	}
	code, err := a.Execute(context.Background(), agent.Expand(tmpl, desc, abs), abs, f)
	if err != nil {
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
