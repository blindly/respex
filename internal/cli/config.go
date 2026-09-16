package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/blindly/respex/internal/config"
)

func configTarget(local bool) (string, string, error) {
	if !local {
		path, err := config.GlobalPath()
		return path, "user", err
	}
	w, err := discover()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(w.root, ".respex", "config.toml"), "project", nil
}

func parseConfigArgs(args []string) (string, bool, error) {
	if len(args) == 0 {
		return "", false, errors.New("usage: respex config <init|path|edit|validate|show> [--local]")
	}
	action := args[0]
	if action != "init" && action != "path" && action != "edit" && action != "validate" && action != "show" {
		return "", false, errors.New("usage: respex config <init|path|edit|validate|show> [--local]")
	}
	local := len(args) == 2 && args[1] == "--local"
	if len(args) > 1 && !local {
		return "", false, errors.New("usage: respex config <init|path|edit|validate|show> [--local]")
	}
	if action == "show" && local {
		return "", false, errors.New("respex config show displays the effective merged config and does not accept --local")
	}
	return action, local, nil
}

func initializeConfig(path, level string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s config already exists: %s", level, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(configTemplate), 0o644)
}

func validateConfig(path string, local bool) error {
	if local {
		global, _ := config.GlobalPath()
		_, err := config.Load(global, path)
		return err
	}
	_, err := config.Load(path, "")
	return err
}

func runConfig(args []string, out, errOut io.Writer) int {
	action, local, err := parseConfigArgs(args)
	if err != nil {
		return fail(errOut, err)
	}
	if action == "show" {
		return runConfigShow(out, errOut)
	}
	path, level, err := configTarget(local)
	if err != nil {
		return fail(errOut, err)
	}
	switch action {
	case "path":
		fmt.Fprintln(out, path)
		return 0
	case "init":
		if err := initializeConfig(path, level); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintf(out, "created %s config: %s\n", level, path)
		return 0
	case "validate":
		if err := validateConfig(path, local); err != nil {
			return fail(errOut, err)
		}
		fmt.Fprintf(out, "%s config is valid: %s\n", level, path)
		return 0
	case "edit":
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := initializeConfig(path, level); err != nil {
				return fail(errOut, err)
			}
		} else if err != nil {
			return fail(errOut, err)
		}
		editor, err := configEditor(path, local)
		if err != nil {
			return fail(errOut, err)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		cmd := exec.CommandContext(ctx, editor[0], append(editor[1:], path)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fail(errOut, fmt.Errorf("editor: %w", err))
		}
		if err := validateConfig(path, local); err != nil {
			return fail(errOut, fmt.Errorf("edited %s config is invalid and was preserved: %w", level, err))
		}
		fmt.Fprintf(out, "updated %s config: %s\n", level, path)
		return 0
	}
	return fail(errOut, errors.New("unreachable config action"))
}

func configEditor(path string, local bool) ([]string, error) {
	global := path
	project := ""
	if local {
		global, _ = config.GlobalPath()
		project = path
	}
	cfg, err := config.Load(global, project)
	if err == nil {
		return resolveEditor(cfg.Editor)
	}
	return resolveEditor(nil)
}
