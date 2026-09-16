package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	Draft    string `toml:"draft"`
	Baseline string `toml:"baseline"`
	Refine   string `toml:"refine"`
	Apply    string `toml:"apply"`
}

// Config is the root configuration schema: spec file, agent settings, and
// prompt overrides.
type Config struct {
	Spec         string        `toml:"spec"`
	Editor       []string      `toml:"editor"`
	AgentTimeout time.Duration `toml:"-"`
	Agent        Agent         `toml:"agent"`
	Prompts      Prompts       `toml:"prompts"`
}

type fileConfig struct {
	Spec         string   `toml:"spec"`
	Editor       []string `toml:"editor"`
	AgentTimeout string   `toml:"agent_timeout"`
	ApplyTimeout string   `toml:"apply_timeout"`
	Agent        Agent    `toml:"agent"`
	Prompts      Prompts  `toml:"prompts"`
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{Spec: "SPEC.md", AgentTimeout: time.Hour, Agent: Agent{Delivery: "argv"}}
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
		var raw fileConfig
		if err := toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(&raw); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", p, err)
		}
		c := Config{Spec: raw.Spec, Editor: raw.Editor, Agent: raw.Agent, Prompts: raw.Prompts}
		timeoutRaw := raw.AgentTimeout
		if timeoutRaw == "" {
			timeoutRaw = raw.ApplyTimeout
		}
		if timeoutRaw != "" {
			c.AgentTimeout, err = time.ParseDuration(timeoutRaw)
			if err != nil || c.AgentTimeout <= 0 {
				return Config{}, fmt.Errorf("parse config %s: invalid agent timeout %q", p, timeoutRaw)
			}
		}
		merge(&cfg, c)
	}
	return cfg, nil
}

func merge(dst *Config, src Config) {
	if src.Spec != "" {
		dst.Spec = src.Spec
	}
	if len(src.Editor) > 0 {
		dst.Editor = src.Editor
	}
	if src.AgentTimeout > 0 {
		dst.AgentTimeout = src.AgentTimeout
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
	if src.Prompts.Baseline != "" {
		dst.Prompts.Baseline = src.Prompts.Baseline
	}
	if src.Prompts.Refine != "" {
		dst.Prompts.Refine = src.Prompts.Refine
	}
	if src.Prompts.Apply != "" {
		dst.Prompts.Apply = src.Prompts.Apply
	}
}

// GlobalPath returns the user-level config path. XDG_CONFIG_HOME is honored
// when set (including on macOS); otherwise os.UserConfigDir is used, which
// gives %AppData% on Windows and ~/Library/Application Support on macOS.
func GlobalPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "respex", "config.toml"), nil
}
