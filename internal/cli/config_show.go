package cli

import (
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

func runConfigShow(out, errOut io.Writer) int {
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
	fmt.Fprintf(out, "spec:          %s  (%s)\n", cfg.Spec, configSource(userKeys, projectKeys, "", "spec"))
	fmt.Fprintf(out, "editor:        %s  (%s)\n", formatArgv(cfg.Editor), configSource(userKeys, projectKeys, "", "editor"))
	fmt.Fprintf(out, "agent_timeout: %s  (%s)\n", cfg.AgentTimeout, configSource(userKeys, projectKeys, "", "agent_timeout"))
	fmt.Fprintf(out, "agent.command: %s  (%s)\n", formatArgv(cfg.Agent.Command), configSource(userKeys, projectKeys, "agent", "command"))
	fmt.Fprintf(out, "agent.delivery: %s  (%s)\n", cfg.Agent.Delivery, configSource(userKeys, projectKeys, "agent", "delivery"))
	fmt.Fprintf(out, "agent.env:     %s  (%s)\n", redactEnv(cfg.Agent.Env), configSource(userKeys, projectKeys, "agent", "env"))
	return 0
}
