package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindly/respex/internal/agent"
)

func runDoctor(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("doctor", errOut)
	jsonOutput := fs.Bool("json", false, "output results as JSON")
	agentCheck := fs.Bool("agent-check", false, "run a short test prompt through the configured agent")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, errors.New("usage: respex doctor [--json] [--agent-check]"))
	}
	failed := false
	var checks []map[string]string
	report := func(status, name, detail string) {
		checks = append(checks, map[string]string{"status": status, "name": name, "detail": detail})
		if !*jsonOutput {
			fmt.Fprintf(out, "%-4s  %-12s %s\n", status, name, detail)
		}
		if status == "FAIL" {
			failed = true
		}
	}
	global, _, err := configTarget(false)
	if err != nil {
		report("FAIL", "user config", err.Error())
	} else if err := validateConfig(global, false); err != nil {
		report("FAIL", "user config", err.Error())
	} else {
		report("PASS", "user config", global)
	}
	w, err := discover()
	if errors.Is(err, errNotProject) {
		report("WARN", "project", "not in a respex project")
	} else if err != nil {
		report("FAIL", "project", err.Error())
	} else {
		local := filepath.Join(w.root, ".respex", "config.toml")
		if err := validateConfig(local, true); err != nil {
			report("FAIL", "project config", err.Error())
		} else {
			report("PASS", "project config", local)
		}
		if len(w.cfg.Agent.Command) == 0 {
			report("WARN", "agent", "not configured")
		} else if path, err := exec.LookPath(w.cfg.Agent.Command[0]); err != nil {
			report("FAIL", "agent", err.Error())
		} else {
			delivery := w.cfg.Agent.Delivery
			if delivery == "" {
				delivery = agent.DeliveryArgv
			}
			_, _, buildErr := agent.Build(w.cfg.Agent.Command, delivery, "doctor probe", filepath.Join(w.root, "SPEC.md"))
			if buildErr != nil {
				report("FAIL", "agent", buildErr.Error())
			} else if *agentCheck {
				a := agent.Adapter{Command: w.cfg.Agent.Command, Delivery: delivery, Env: w.cfg.Agent.Env, Dir: w.root}
				tmpDir := filepath.Join(w.root, ".respex", "tmp")
				if err := os.MkdirAll(tmpDir, 0o755); err != nil {
					report("FAIL", "agent-check", fmt.Sprintf("create tmp dir: %v", err))
				} else {
					tmp, err := os.CreateTemp(tmpDir, "doctor-agent-check-*.log")
					if err != nil {
						report("FAIL", "agent-check", fmt.Sprintf("create temp log: %v", err))
					} else {
						defer tmp.Close()
						defer os.Remove(tmp.Name())
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						code, err := a.Execute(ctx, "Reply with only the word OK to confirm the agent CLI is reachable.", filepath.Join(w.root, "SPEC.md"), tmp)
						cancel()
						if err != nil {
							report("FAIL", "agent-check", err.Error())
						} else if code != 0 {
							report("FAIL", "agent-check", fmt.Sprintf("exit %d", code))
						} else {
							report("PASS", "agent-check", "agent responded to test prompt")
						}
					}
				}
			} else {
				report("PASS", "agent", path)
			}
		}
		if len(w.cfg.Verify.Commands) > 0 || w.cfg.Verify.Audit {
			verifyFailures := []string{}
			for _, argv := range w.cfg.Verify.Commands {
				if len(argv) == 0 {
					verifyFailures = append(verifyFailures, "empty command")
					continue
				}
				if _, err := exec.LookPath(argv[0]); err != nil {
					verifyFailures = append(verifyFailures, err.Error())
				}
			}
			detail := fmt.Sprintf("%d command(s)", len(w.cfg.Verify.Commands))
			if w.cfg.Verify.Audit {
				detail += " + agent audit"
				if len(w.cfg.Agent.Command) == 0 {
					verifyFailures = append(verifyFailures, "audit = true requires [agent] command")
				} else if _, err := exec.LookPath(w.cfg.Agent.Command[0]); err != nil {
					verifyFailures = append(verifyFailures, "audit agent: "+err.Error())
				}
			}
			if len(verifyFailures) > 0 {
				report("FAIL", "verify", strings.Join(verifyFailures, "; "))
			} else {
				report("PASS", "verify", detail)
			}
		}
		if editor, err := resolveEditor(w.cfg.Editor); err != nil {
			report("WARN", "editor", err.Error())
		} else if path, err := exec.LookPath(editor[0]); err != nil {
			report("WARN", "editor", err.Error())
		} else {
			report("PASS", "editor", path)
		}
		st, err := w.openState()
		if err != nil {
			report("FAIL", "state", err.Error())
		} else {
			schema, schemaErr := st.SchemaVersion()
			st.Close()
			if schemaErr != nil {
				report("FAIL", "state", schemaErr.Error())
			} else {
				report("PASS", "state", fmt.Sprintf("schema v%d", schema))
			}
		}
		paths, pathsErr := w.specPaths()
		if pathsErr != nil {
			report("FAIL", "spec files", pathsErr.Error())
		} else {
			missing := []string{}
			for _, p := range paths {
				if _, err := os.Stat(filepath.Join(w.root, filepath.FromSlash(p))); err != nil {
					missing = append(missing, p)
				}
			}
			if len(missing) > 0 {
				report("FAIL", "spec files", "missing: "+strings.Join(missing, ", "))
			} else {
				report("PASS", "spec files", fmt.Sprintf("%d file(s)", len(paths)))
			}
		}
		lock, locked, lockErr := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
		if lockErr != nil {
			report("FAIL", "operation", lockErr.Error())
		} else if !locked {
			report("WARN", "operation", "another operation is running")
		} else {
			lock.Close()
			report("PASS", "operation", "idle")
		}
	}
	report("PASS", "version", version)
	if *jsonOutput {
		if err := json.NewEncoder(out).Encode(map[string]any{"ok": !failed, "checks": checks}); err != nil {
			return fail(errOut, err)
		}
	}
	if failed {
		if !*jsonOutput {
			fmt.Fprintln(errOut, "respex: doctor found failures")
		}
		return 1
	}
	return 0
}
