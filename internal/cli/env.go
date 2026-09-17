package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/blindly/respex/internal/agent"
	"github.com/blindly/respex/internal/config"
	"github.com/blindly/respex/internal/state"
)

// workspace is the resolved environment for a command.
type workspace struct {
	root string
	cfg  config.Config
}

var errNotProject = errors.New("not a respex project — run `respex init`")

// discover walks up from the working directory looking for .respex/.
func discover() (*workspace, error) {
	p, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if fi, err := os.Stat(filepath.Join(p, ".respex")); err == nil && fi.IsDir() {
			return loadWorkspace(p)
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("checking %s/.respex: %w", p, err)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return nil, errNotProject
		}
		p = parent
	}
}

func loadWorkspace(root string) (*workspace, error) {
	cfg, err := loadConfig(root)
	if err != nil {
		return nil, err
	}
	return &workspace{root: root, cfg: cfg}, nil
}

// loadConfig merges the user-level config, then the project config, over
// defaults. A GlobalPath() error (e.g. HOME unset) silently degrades to
// project-config-only.
func loadConfig(root string) (config.Config, error) {
	global, gerr := config.GlobalPath()
	if gerr != nil {
		global = ""
	}
	return config.Load(global, filepath.Join(root, ".respex", "config.toml"))
}

// specPath joins cfg.Spec under root; cfg.Spec is anchored under root — an
// absolute path is joined textually per spec §3 root-anchoring.
func (w *workspace) specPath() string { return filepath.Join(w.root, w.cfg.Spec) }

// notesPath joins cfg.Notes under root. The default is .respex/notes.md.
func (w *workspace) notesPath() string { return filepath.Join(w.root, w.cfg.Notes) }

func (w *workspace) openState() (*state.DB, error) {
	return state.Open(filepath.Join(w.root, ".respex", "state.db"))
}

// adapter resolves the configured agent; the second return is its display name.
func (w *workspace) adapter() (agent.Adapter, string, error) {
	if len(w.cfg.Agent.Command) == 0 {
		return agent.Adapter{}, "", errors.New(
			"no agent configured — set [agent] command in .respex/config.toml")
	}
	delivery := w.cfg.Agent.Delivery
	// Defensive: Defaults() already sets argv; merge cannot unset it.
	if delivery == "" {
		delivery = agent.DeliveryArgv
	}
	return agent.Adapter{
		Command:  w.cfg.Agent.Command,
		Delivery: delivery,
		Env:      w.cfg.Agent.Env,
		Dir:      w.root,
	}, w.cfg.Agent.Command[0], nil
}

// absSpecPath returns the absolute spec path.
func (w *workspace) absSpecPath() string {
	abs, err := filepath.Abs(w.specPath())
	if err != nil {
		return w.specPath()
	}
	return abs
}
