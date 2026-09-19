package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
)

// verifyCommandResult is one entry in a verification run's detail JSON.
type verifyCommandResult struct {
	Command []string `json:"command"`
	Status  string   `json:"status"` // passed | failed | error | timed_out | interrupted | skipped
	Exit    int      `json:"exit"`
}

// verifyAuditResult summarizes the optional agent audit inside a run. When
// audit is configured but the command checks fail first, Status is "skipped".
type verifyAuditResult struct {
	Status  string `json:"status"` // passed | failed | error | timed_out | interrupted | skipped
	Exit    int    `json:"exit"`
	Agent   string `json:"agent,omitempty"`
	Verdict string `json:"verdict,omitempty"` // yes | no, empty when absent
	Note    string `json:"note,omitempty"`
}

// verifyDetail is the detail JSON stored on a verifications row.
type verifyDetail struct {
	Results []verifyCommandResult `json:"results"`
	Audit   *verifyAuditResult    `json:"audit,omitempty"`
}

// verificationRun summarizes a finished verification run.
type verificationRun struct {
	ID      int64
	LogPath string
	Outcome string
	Results []verifyCommandResult
	Audit   *verifyAuditResult `json:"audit,omitempty"`
}

func runVerify(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("verify", errOut)
	jsonOutput := fs.Bool("json", false, "output results as JSON")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, errors.New("usage: respex verify [--json]"))
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
	last, err := st.LatestVersion()
	if err != nil {
		return fail(errOut, err)
	}
	if last == nil {
		return fail(errOut, errors.New("no committed versions yet — run `respex commit` first"))
	}
	// Verify checks the working tree against a committed version; a dirty
	// spec would make the run's meaning ambiguous. Same gate as apply.
	workingHash, err := w.workingSpecHash()
	if err != nil || workingHash != last.Hash {
		return fail(errOut, errors.New("spec changed since last commit — review with `respex diff`, then `respex commit`"))
	}
	if len(w.cfg.Verify.Commands) == 0 && !w.cfg.Verify.Audit {
		return fail(errOut, errors.New("no [verify] checks configured — add commands = [[\"go\", \"test\", \"./...\"]] or audit = true under [verify] in .respex/config.toml"))
	}
	if w.cfg.Verify.Audit && len(w.cfg.Agent.Command) == 0 {
		return fail(errOut, errors.New("verify audit requires an agent — set [agent] command in .respex/config.toml"))
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, err)
	}
	if !locked {
		return fail(errOut, errors.New("another respex operation is running — wait for it to finish"))
	}
	defer lock.Close()
	if err := st.MarkUnfinishedVerificationsStale(); err != nil {
		return fail(errOut, err)
	}
	display := out
	if *jsonOutput {
		display = io.Discard
	}
	run, code := runVerification(w, st, last, display, errOut)
	if *jsonOutput {
		payload := map[string]any{"version": last.ID, "outcome": run.Outcome, "results": run.Results, "log": run.LogPath}
		if run.Audit != nil {
			payload["audit"] = run.Audit
		}
		if err := json.NewEncoder(out).Encode(payload); err != nil {
			return fail(errOut, err)
		}
	}
	return code
}

// runVerification executes the configured [verify] checks against the working
// tree for the committed version last and records a verifications row.
// Commands run in order; the run stops at the first failing command and the
// rest are recorded as skipped. When [verify] audit is enabled and the
// commands all pass, the configured agent audits conformance against an
// immutable snapshot of the committed spec. The caller must hold the
// operation lock and keep st open. The returned exit code is 0 only when
// every check passed.
func runVerification(w *workspace, st *state.DB, last *state.SpecVersion, display, errOut io.Writer) (verificationRun, int) {
	run := verificationRun{Results: []verifyCommandResult{}}
	versionID := last.ID
	sha, dirty := gitInfo(w.root)

	auditAgent := ""
	var auditAdapter agent.Adapter
	if w.cfg.Verify.Audit {
		a, name, err := w.adapter()
		if err != nil {
			return run, fail(errOut, err)
		}
		auditAdapter, auditAgent = a, name
	}
	id, err := st.InsertVerification(versionID, auditAgent, time.Now())
	if err != nil {
		return run, fail(errOut, err)
	}
	logRel := fmt.Sprintf(".respex/logs/%d-verify.log", id)
	logAbs := filepath.Join(w.root, filepath.FromSlash(logRel))
	if err := os.MkdirAll(filepath.Dir(logAbs), 0o755); err != nil {
		return run, fail(errOut, err)
	}
	if err := st.SetVerificationLogPath(id, logRel); err != nil {
		return run, fail(errOut, err)
	}
	run.LogPath = logRel
	f, err := os.Create(logAbs)
	if err != nil {
		return run, fail(errOut, err)
	}
	defer f.Close()
	shaLabel := sha
	if shaLabel == "" {
		shaLabel = "(none)"
	}
	fmt.Fprintf(f, "verify v%d started %s (git %s, tree dirty: %t)\n\n",
		versionID, time.Now().UTC().Format(time.RFC3339), shaLabel, dirty)

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, w.cfg.Verify.Timeout)
	defer cancel()

	for i, argv := range w.cfg.Verify.Commands {
		if i > 0 {
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "$ %s\n", strings.Join(argv, " "))
		status, exit, execErr := runVerifyCommand(ctx, argv, w.root, w.cfg.Verify.Env, f)
		if execErr != nil {
			fmt.Fprintf(f, "error: %v\n", execErr)
		}
		run.Results = append(run.Results, verifyCommandResult{Command: argv, Status: status, Exit: exit})
		if status != "passed" {
			run.Outcome = status
			for _, skipped := range w.cfg.Verify.Commands[i+1:] {
				run.Results = append(run.Results, verifyCommandResult{Command: skipped, Status: "skipped"})
			}
			break
		}
	}
	if run.Outcome == "" {
		run.Outcome = "passed"
	}
	if w.cfg.Verify.Audit {
		audit := &verifyAuditResult{Status: "skipped", Agent: auditAgent}
		if run.Outcome == "passed" {
			audit = runVerifyAudit(ctx, w, auditAdapter, last, f)
			run.Outcome = audit.Status
		}
		run.Audit = audit
	}
	detail, err := json.Marshal(verifyDetail{Results: run.Results, Audit: run.Audit})
	if err != nil {
		return run, fail(errOut, err)
	}
	if err := st.FinishVerificationWithOutcome(id, run.Outcome, detail, time.Now()); err != nil {
		return run, fail(errOut, err)
	}

	for _, r := range run.Results {
		switch r.Status {
		case "passed":
			fmt.Fprintf(display, "[pass] %s\n", strings.Join(r.Command, " "))
		case "skipped":
			fmt.Fprintf(display, "[skip] %s\n", strings.Join(r.Command, " "))
		case "failed":
			fmt.Fprintf(display, "[fail] %s (exit %d)\n", strings.Join(r.Command, " "), r.Exit)
		default:
			fmt.Fprintf(display, "[fail] %s (%s)\n", strings.Join(r.Command, " "), r.Status)
		}
	}
	if run.Audit != nil {
		switch run.Audit.Status {
		case "passed":
			fmt.Fprintf(display, "[audit] pass — %s reported conformance\n", run.Audit.Agent)
		case "skipped":
			fmt.Fprintln(display, "[audit] skip")
		case "failed":
			fmt.Fprintln(display, "[audit] fail — agent reported non-conformance")
		default:
			if run.Audit.Note != "" {
				fmt.Fprintf(display, "[audit] fail — %s\n", run.Audit.Note)
			} else {
				fmt.Fprintf(display, "[audit] fail (%s)\n", run.Audit.Status)
			}
		}
	}
	switch run.Outcome {
	case "passed":
		fmt.Fprintf(display, "verifying v%d: passed — log: %s\n", versionID, run.LogPath)
		return run, 0
	case "timed_out":
		fmt.Fprintf(display, "verifying v%d: timed out after %s — log: %s\n", versionID, w.cfg.Verify.Timeout, run.LogPath)
	case "interrupted":
		fmt.Fprintf(display, "verification interrupted — log: %s\n", run.LogPath)
	default:
		fmt.Fprintf(display, "verifying v%d: %s — log: %s\n", versionID, run.Outcome, run.LogPath)
	}
	return run, 1
}

// conformsRe matches a verdict at the end of a line, such as "CONFORMS: yes"
// with optional punctuation or Markdown emphasis around it. An agent that
// merely echoes the prompt can glue the verdict onto the last echoed line, so
// only the line end is anchored; instruction lines cannot match because they
// continue after the marker words (…when it does not.). The last match wins.
var conformsRe = regexp.MustCompile(`(?i)CONFORMS:?\W*(yes|no)\W*$`)

// parseConformsVerdict returns the last CONFORMS verdict ("yes" or "no") in
// the agent output, or "" when none is present.
func parseConformsVerdict(out string) string {
	verdict := ""
	for _, line := range strings.Split(out, "\n") {
		if m := conformsRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			verdict = strings.ToLower(m[1])
		}
	}
	return verdict
}

// runVerifyAudit asks the configured agent to audit conformance against an
// immutable snapshot of the committed spec. Agent output is appended to the
// run log; a copy is scanned for the CONFORMS verdict. The caller supplies
// the run's context, so the audit shares the whole run's timeout budget.
func runVerifyAudit(ctx context.Context, w *workspace, a agent.Adapter, last *state.SpecVersion, log io.Writer) *verifyAuditResult {
	res := &verifyAuditResult{Status: "error", Exit: -1, Agent: a.Command[0]}
	master, err := spec.CleanSpecPath(w.cfg.Spec)
	if err != nil {
		res.Note = err.Error()
		fmt.Fprintf(log, "$ audit %s\nerror: %v\n", res.Agent, err)
		return res
	}
	absSpec, cleanup, err := materializeSnapshot(w.root, last.ID, versionFiles(last.Content, master))
	if err != nil {
		res.Note = err.Error()
		fmt.Fprintf(log, "$ audit %s\nerror: %v\n", res.Agent, err)
		return res
	}
	defer cleanup()
	tmpl := agent.PromptVerify
	if w.cfg.Prompts.Verify != "" {
		tmpl = w.cfg.Prompts.Verify
	}
	instr := agent.Expand(tmpl, "", absSpec)
	fmt.Fprintf(log, "\n$ audit %s\n", res.Agent)
	var reply bytes.Buffer
	code, execErr := a.Execute(ctx, instr, absSpec, io.MultiWriter(log, &reply))
	res.Exit = code
	switch {
	case errors.Is(execErr, context.DeadlineExceeded):
		res.Status = "timed_out"
		fmt.Fprintln(log, "error: audit timed out")
		return res
	case errors.Is(execErr, context.Canceled):
		res.Status = "interrupted"
		fmt.Fprintln(log, "error: audit interrupted")
		return res
	case execErr != nil:
		res.Note = execErr.Error()
		fmt.Fprintf(log, "error: %v\n", execErr)
		return res
	}
	res.Verdict = parseConformsVerdict(reply.String())
	switch {
	case res.Verdict == "yes" && code == 0:
		res.Status = "passed"
	case res.Verdict == "no":
		res.Status = "failed"
	case res.Verdict == "yes":
		res.Note = fmt.Sprintf("agent reported conformance but exited %d", code)
	default:
		res.Note = "no CONFORMS: yes/no verdict in agent output"
		if w.cfg.Prompts.Verify != "" {
			res.Note += " — a custom [prompts] verify template must still end with a CONFORMS: yes/no line"
		}
	}
	return res
}

// runVerifyCommand executes one verify command with combined output going to
// the log. It returns the command-level status, the exit code (-1 for non-exit
// errors), and a non-nil error only for start failures and
// timeout/interruption — a non-zero exit is a normal "failed" result.
func runVerifyCommand(ctx context.Context, argv []string, dir string, extraEnv []string, log io.Writer) (string, int, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = log
	cmd.Stderr = log
	// Bound the wait so orphaned descendants holding the output pipes
	// cannot hang it past the deadline (same policy as the agent adapter).
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return "error", -1, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		exit := 0
		if err != nil {
			var ee *exec.ExitError
			switch {
			case errors.As(err, &ee):
				exit = ee.ExitCode()
			case errors.Is(err, exec.ErrWaitDelay):
				// The command exited; a descendant held the pipes.
				exit = cmd.ProcessState.ExitCode()
			default:
				return "error", -1, err
			}
		}
		if exit == 0 {
			return "passed", 0, nil
		}
		return "failed", exit, nil
	case <-ctx.Done():
		<-done
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "timed_out", -1, ctx.Err()
		}
		return "interrupted", -1, ctx.Err()
	}
}

// gitInfo reports the HEAD commit and working-tree dirtiness for the
// repository at root. It degrades gracefully: when root is not a git
// repository or git is unavailable, both values are zero values. The values
// are recorded on verification rows for audit only; they never gate a run.
func gitInfo(root string) (string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", false
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	sha := strings.TrimSpace(string(out))
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = root
	statusOut, err := statusCmd.Output()
	if err != nil {
		return sha, false
	}
	return sha, len(bytes.TrimSpace(statusOut)) > 0
}
