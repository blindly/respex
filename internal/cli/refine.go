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

	"respex/internal/agent"
	"respex/internal/spec"
)

func runRefine(args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex refine", args[0]))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	before, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	beforeHash := spec.Hash(before)
	a, name, err := w.adapter()
	if err != nil {
		return fail(errOut, err)
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

	tmpl := agent.PromptRefine
	if w.cfg.Prompts.Refine != "" {
		tmpl = w.cfg.Prompts.Refine
	}
	absSpec := w.absSpecPath()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	code, err := a.Execute(ctx, agent.Expand(tmpl, "", absSpec), absSpec, f)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintf(out, "refine interrupted — log: %s\n", logPath)
			return 1
		}
		return fail(errOut, err)
	}
	if code != 0 {
		return fail(errOut, fmt.Errorf("refine failed (exit %d) — log: %s", code, logPath))
	}
	after, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, fmt.Errorf("agent removed the spec — log: %s: %w", logPath, err))
	}
	afterHash := spec.Hash(after)
	if afterHash == beforeHash {
		fmt.Fprintln(out, "spec unchanged")
		return 0
	}
	fmt.Fprintf(out, "spec updated via %s (%s… → %s…) — log: %s\n",
		name, beforeHash[:8], afterHash[:8], logPath)
	return 0
}
