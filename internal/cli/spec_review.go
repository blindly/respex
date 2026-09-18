package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/blindly/respex/internal/agent"
)

func runSpecReview(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("spec review", errOut)
	withNotes := fs.Bool("notes", false, "also read project notes as context")
	jsonOut := fs.Bool("json", false, "output results as JSON")
	fs.Bool("no-progress", false, "deprecated; review output is always streamed")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() > 1 {
		return fail(errOut, fmt.Errorf("usage: respex spec review [--notes] [--json] [feature]"))
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
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex spec review [--notes] [--json] [feature]", featureName))
	}
	specPath, err := w.resolveFeature(featureName)
	if err != nil {
		return fail(errOut, err)
	}

	delivery := w.cfg.Agent.Delivery
	if delivery == "" {
		delivery = agent.DeliveryArgv
	}
	if len(w.cfg.Agent.Command) == 0 {
		return fail(errOut, fmt.Errorf("no agent configured; set [agent] command in .respex/config.toml"))
	}
	if _, _, err := agent.Build(w.cfg.Agent.Command, delivery, "review probe", specPath); err != nil {
		return fail(errOut, err)
	}

	logsDir := filepath.Join(w.root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fail(errOut, err)
	}
	f, err := os.CreateTemp(logsDir, time.Now().UTC().Format("20060102T150405Z")+"-review-*.log")
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()

	a := agent.Adapter{Command: w.cfg.Agent.Command, Delivery: delivery, Env: w.cfg.Agent.Env, Dir: w.root}
	instr := agent.Expand(agent.PromptSpecReview, "", specPath)
	if *withNotes {
		notesPath := w.notesPath()
		if _, err := os.Stat(notesPath); err == nil {
			instr += fmt.Sprintf("\n\nAlso read the project notes at %s for additional context, but do not modify the notes file.", notesPath)
		}
	}

	var reviewOut io.Writer = f
	if !*jsonOut {
		fmt.Fprintf(out, "review log: %s\n\n", f.Name())
		reviewOut = io.MultiWriter(out, f)
	}

	ctx, cancel := context.WithTimeout(context.Background(), w.cfg.AgentTimeout)
	defer cancel()
	code, err := a.Execute(ctx, instr, specPath, reviewOut)
	if err != nil {
		return fail(errOut, err)
	}
	if code != 0 {
		return fail(errOut, fmt.Errorf("review failed (exit %d); see %s", code, f.Name()))
	}

	critique, err := os.ReadFile(f.Name())
	if err != nil {
		return fail(errOut, err)
	}

	if *jsonOut {
		payload := map[string]any{
			"ok":       true,
			"critique": string(critique),
			"log":      f.Name(),
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fail(errOut, err)
		}
		return 0
	}

	if len(bytes.TrimSpace(critique)) == 0 {
		fmt.Fprintln(out, "agent returned no critique")
	}
	return 0
}
