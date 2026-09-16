package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

func runDoctor(args []string, out, errOut io.Writer) int {
	jsonOutput := len(args) == 1 && args[0] == "--json"
	if len(args) != 0 && !jsonOutput {
		return fail(errOut, errors.New("usage: respex doctor [--json]"))
	}
	failed := false
	var checks []map[string]string
	report := func(status, name, detail string) {
		checks = append(checks, map[string]string{"status": status, "name": name, "detail": detail})
		if !jsonOutput {
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
			report("PASS", "agent", path)
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
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(map[string]any{"ok": !failed, "checks": checks}); err != nil {
			return fail(errOut, err)
		}
	}
	if failed {
		if !jsonOutput {
			fmt.Fprintln(errOut, "respex: doctor found failures")
		}
		return 1
	}
	return 0
}
