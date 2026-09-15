package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Agent configures how prompts reach the agent process. Empty array/string
// means "not set" — later files cannot unset earlier values.
type Agent struct {
	Command  []string `toml:"command"`
	Delivery string   `toml:"delivery"`
	Env      []string `toml:"env"`
}

// Prompts holds per-stage prompt overrides. Empty string means "not set" —
// later files cannot unset earlier values.
type Prompts struct {
	Draft  string `toml:"draft"`
	Refine string `toml:"refine"`
	Apply  string `toml:"apply"`
}

// Config is the root configuration schema: spec file, agent settings, and
// prompt overrides.
type Config struct {
	Spec    string  `toml:"spec"`
	Agent   Agent   `toml:"agent"`
	Prompts Prompts `toml:"prompts"`
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{Spec: "SPEC.md", Agent: Agent{Delivery: "argv"}}
}

// Load merges global (optional) then project (optional) files over Defaults:
// top-level keys from later files win; [agent]/[prompts] keys merge
// individually; arrays replace wholesale. Missing files are skipped. An empty
// array/string means "not set" — later files cannot unset earlier values.
// Unknown keys are a parse error.
func Load(globalPath, projectPath string) (Config, error) {
	cfg := Defaults()
	for _, p := range []string{globalPath, projectPath} {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", p, err)
		}
		var c Config
		if err := toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(&c); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", p, err)
		}
		merge(&cfg, c)
	}
	return cfg, nil
}

func merge(dst *Config, src Config) {
	if src.Spec != "" {
		dst.Spec = src.Spec
	}
	if len(src.Agent.Command) > 0 {
		dst.Agent.Command = src.Agent.Command
	}
	if src.Agent.Delivery != "" {
		dst.Agent.Delivery = src.Agent.Delivery
	}
	if len(src.Agent.Env) > 0 {
		dst.Agent.Env = src.Agent.Env
	}
	if src.Prompts.Draft != "" {
		dst.Prompts.Draft = src.Prompts.Draft
	}
	if src.Prompts.Refine != "" {
		dst.Prompts.Refine = src.Prompts.Refine
	}
	if src.Prompts.Apply != "" {
		dst.Prompts.Apply = src.Prompts.Apply
	}
}

// GlobalPath returns the user-level config path: os.UserConfigDir honors
// XDG_CONFIG_HOME on Unix (typically ~/.config/respex/config.toml), %AppData%
// on Windows, and ~/Library/Application Support on macOS.
func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "respex", "config.toml"), nil
}
