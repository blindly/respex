package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

type Agent struct {
	Command  []string `toml:"command"`
	Delivery string   `toml:"delivery"`
	Env      []string `toml:"env"`
}

type Prompts struct {
	Draft  string `toml:"draft"`
	Refine string `toml:"refine"`
	Apply  string `toml:"apply"`
}

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
// individually; arrays replace wholesale. Missing files are skipped.
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
			return cfg, fmt.Errorf("read config %s: %w", p, err)
		}
		var c Config
		if err := toml.Unmarshal(b, &c); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", p, err)
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

// GlobalPath returns the user-level config path (XDG on Unix, %AppData% on Windows).
func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "respex", "config.toml"), nil
}
