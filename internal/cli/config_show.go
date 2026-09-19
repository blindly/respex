package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/blindly/respex/internal/config"
)

func configKeys(path string) (map[string]any, error) {
	if path == "" {
		return map[string]any{}, nil
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var keys map[string]any
	if err := toml.Unmarshal(content, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func configSource(user, project map[string]any, section, key string) string {
	has := func(values map[string]any) bool {
		if section == "" {
			_, ok := values[key]
			if !ok && key == "agent_timeout" {
				_, ok = values["apply_timeout"]
			}
			return ok
		}
		table, ok := values[section].(map[string]any)
		if !ok {
			return false
		}
		_, ok = table[key]
		return ok
	}
	if has(project) {
		return "project"
	}
	if has(user) {
		return "user"
	}
	return "default"
}

func formatArgv(argv []string) string {
	if len(argv) == 0 {
		return "(not configured)"
	}
	parts := make([]string, len(argv))
	for i, arg := range argv {
		parts[i] = fmt.Sprintf("%q", arg)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func redactEnv(env []string) string {
	if len(env) == 0 {
		return "[]"
	}
	redacted := make([]string, len(env))
	for i, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		redacted[i] = name + "=<redacted>"
	}
	return "[" + strings.Join(redacted, ", ") + "]"
}

func runConfigShow(out, errOut io.Writer, jsonOutput bool) int {
	global, err := config.GlobalPath()
	if err != nil {
		return fail(errOut, err)
	}
	project := ""
	if w, discoverErr := discover(); discoverErr == nil {
		project = filepath.Join(w.root, ".respex", "config.toml")
	} else if !errors.Is(discoverErr, errNotProject) {
		return fail(errOut, discoverErr)
	}
	cfg, err := config.Load(global, project)
	if err != nil {
		return fail(errOut, err)
	}
	userKeys, err := configKeys(global)
	if err != nil {
		return fail(errOut, err)
	}
	projectKeys, err := configKeys(project)
	if err != nil {
		return fail(errOut, err)
	}
	if jsonOutput {
		value := func(v any, section, key string) map[string]any {
			return map[string]any{"value": v, "source": configSource(userKeys, projectKeys, section, key)}
		}
		payload := map[string]any{
			"spec": value(cfg.Spec, "", "spec"), "spec_files": value(cfg.SpecFiles, "", "spec_files"),
			"notes":  value(cfg.Notes, "", "notes"),
			"editor": value(cfg.Editor, "", "editor"), "pager": value(cfg.Pager, "", "pager"),
			"agent_timeout": value(cfg.AgentTimeout.String(), "", "agent_timeout"),
			"agent_command": value(cfg.Agent.Command, "agent", "command"), "agent_delivery": value(cfg.Agent.Delivery, "agent", "delivery"),
			"agent_env":       value(redactEnv(cfg.Agent.Env), "agent", "env"),
			"verify_commands": value(cfg.Verify.Commands, "verify", "commands"), "verify_timeout": value(cfg.Verify.Timeout.String(), "verify", "timeout"),
			"verify_env":   value(redactEnv(cfg.Verify.Env), "verify", "env"),
			"verify_audit": value(cfg.Verify.Audit, "verify", "audit"),
		}
		if err := json.NewEncoder(out).Encode(payload); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	fmt.Fprintf(out, "spec:          %s  (%s)\n", cfg.Spec, configSource(userKeys, projectKeys, "", "spec"))
	specFiles := formatArgv(cfg.SpecFiles)
	if len(cfg.SpecFiles) == 0 {
		specFiles = "(none)"
	}
	fmt.Fprintf(out, "spec_files:    %s  (%s)\n", specFiles, configSource(userKeys, projectKeys, "", "spec_files"))
	fmt.Fprintf(out, "notes:         %s  (%s)\n", cfg.Notes, configSource(userKeys, projectKeys, "", "notes"))
	fmt.Fprintf(out, "editor:        %s  (%s)\n", formatArgv(cfg.Editor), configSource(userKeys, projectKeys, "", "editor"))
	fmt.Fprintf(out, "pager:         %s  (%s)\n", formatArgv(cfg.Pager), configSource(userKeys, projectKeys, "", "pager"))
	fmt.Fprintf(out, "agent_timeout: %s  (%s)\n", cfg.AgentTimeout, configSource(userKeys, projectKeys, "", "agent_timeout"))
	fmt.Fprintf(out, "agent.command: %s  (%s)\n", formatArgv(cfg.Agent.Command), configSource(userKeys, projectKeys, "agent", "command"))
	fmt.Fprintf(out, "agent.delivery: %s  (%s)\n", cfg.Agent.Delivery, configSource(userKeys, projectKeys, "agent", "delivery"))
	fmt.Fprintf(out, "agent.env:     %s  (%s)\n", redactEnv(cfg.Agent.Env), configSource(userKeys, projectKeys, "agent", "env"))
	verifyCmds := "(none)"
	if len(cfg.Verify.Commands) > 0 {
		parts := make([]string, len(cfg.Verify.Commands))
		for i, argv := range cfg.Verify.Commands {
			parts[i] = formatArgv(argv)
		}
		verifyCmds = strings.Join(parts, " then ")
	}
	fmt.Fprintf(out, "verify.commands: %s  (%s)\n", verifyCmds, configSource(userKeys, projectKeys, "verify", "commands"))
	fmt.Fprintf(out, "verify.timeout:  %s  (%s)\n", cfg.Verify.Timeout, configSource(userKeys, projectKeys, "verify", "timeout"))
	fmt.Fprintf(out, "verify.env:      %s  (%s)\n", redactEnv(cfg.Verify.Env), configSource(userKeys, projectKeys, "verify", "env"))
	fmt.Fprintf(out, "verify.audit:    %t  (%s)\n", cfg.Verify.Audit, configSource(userKeys, projectKeys, "verify", "audit"))
	return 0
}
