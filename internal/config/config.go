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
	Verify   string `toml:"verify"`
}

// Verify configures the conformance checks run by `respex verify` and after
// `respex apply`. Empty Commands means "not set" — later files cannot unset
// earlier values, and with no commands and audit disabled verification is off.
// Audit makes the configured agent audit conformance against the committed
// spec after the command checks pass; it is also off by default.
type Verify struct {
	Commands [][]string    `toml:"commands"`
	Timeout  time.Duration `toml:"-"`
	Env      []string      `toml:"env"`
	Audit    bool          `toml:"audit"`
}

// Config is the root configuration schema: spec file, agent settings,
// verification commands, and prompt overrides.
type Config struct {
	Spec         string        `toml:"spec"`
	SpecFiles    []string      `toml:"spec_files"`
	Notes        string        `toml:"notes"`
	Editor       []string      `toml:"editor"`
	Pager        []string      `toml:"pager"`
	AgentTimeout time.Duration `toml:"-"`
	Agent        Agent         `toml:"agent"`
	Prompts      Prompts       `toml:"prompts"`
	Verify       Verify        `toml:"verify"`
}

type verifyFile struct {
	Commands [][]string `toml:"commands"`
	Timeout  string     `toml:"timeout"`
	Env      []string   `toml:"env"`
	Audit    bool       `toml:"audit"`
}

type fileConfig struct {
	Spec         string     `toml:"spec"`
	SpecFiles    []string   `toml:"spec_files"`
	Notes        string     `toml:"notes"`
	Editor       []string   `toml:"editor"`
	Pager        []string   `toml:"pager"`
	AgentTimeout string     `toml:"agent_timeout"`
	ApplyTimeout string     `toml:"apply_timeout"`
	Agent        Agent      `toml:"agent"`
	Prompts      Prompts    `toml:"prompts"`
	Verify       verifyFile `toml:"verify"`
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{Spec: "SPEC.md", Notes: ".respex/notes.md", AgentTimeout: time.Hour, Agent: Agent{Delivery: "argv"}, Verify: Verify{Timeout: 30 * time.Minute}}
}

// Load merges global (optional) then project (optional) files over Defaults:
// top-level keys from later files win; [agent]/[prompts]/[verify] keys merge
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
		c := Config{Spec: raw.Spec, SpecFiles: raw.SpecFiles, Notes: raw.Notes, Editor: raw.Editor, Pager: raw.Pager, Agent: raw.Agent, Prompts: raw.Prompts, Verify: Verify{Commands: raw.Verify.Commands, Env: raw.Verify.Env, Audit: raw.Verify.Audit}}
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
		if raw.Verify.Timeout != "" {
			c.Verify.Timeout, err = time.ParseDuration(raw.Verify.Timeout)
			if err != nil || c.Verify.Timeout <= 0 {
				return Config{}, fmt.Errorf("parse config %s: invalid verify timeout %q", p, raw.Verify.Timeout)
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
	if len(src.SpecFiles) > 0 {
		dst.SpecFiles = src.SpecFiles
	}
	if src.Notes != "" {
		dst.Notes = src.Notes
	}
	if len(src.Editor) > 0 {
		dst.Editor = src.Editor
	}
	if len(src.Pager) > 0 {
		dst.Pager = src.Pager
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
	if src.Prompts.Verify != "" {
		dst.Prompts.Verify = src.Prompts.Verify
	}
	if len(src.Verify.Commands) > 0 {
		dst.Verify.Commands = src.Verify.Commands
	}
	if src.Verify.Timeout > 0 {
		dst.Verify.Timeout = src.Verify.Timeout
	}
	if len(src.Verify.Env) > 0 {
		dst.Verify.Env = src.Verify.Env
	}
	if src.Verify.Audit {
		dst.Verify.Audit = true
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
