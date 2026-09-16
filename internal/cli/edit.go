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
	"runtime"

	"github.com/blindly/respex/internal/spec"
)

func resolveEditor(configured []string) ([]string, error) {
	if len(configured) > 0 {
		return configured, nil
	}
	if visual := os.Getenv("VISUAL"); visual != "" {
		return []string{visual}, nil
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return []string{editor}, nil
	}
	if runtime.GOOS == "windows" {
		return []string{"notepad"}, nil
	}
	return nil, errors.New("no editor configured — set editor = [\"code\", \"--wait\"] or set VISUAL/EDITOR")
}

func runEdit(args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex edit", args[0]))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	lock, locked, err := tryApplyLock(filepath.Join(w.root, ".respex", "operation.lock"))
	if err != nil {
		return fail(errOut, fmt.Errorf("acquire operation lock: %w", err))
	}
	if !locked {
		return fail(errOut, errors.New("another apply, refine, restore, or edit is already running in this project"))
	}
	defer lock.Close()
	before, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	editor, err := resolveEditor(w.cfg.Editor)
	if err != nil {
		return fail(errOut, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd := exec.CommandContext(ctx, editor[0], append(editor[1:], w.absSpecPath())...)
	cmd.Dir = w.root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return fail(errOut, errors.New("edit interrupted"))
		}
		return fail(errOut, fmt.Errorf("editor: %w", err))
	}
	after, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	beforeHash, afterHash := spec.Hash(before), spec.Hash(after)
	if beforeHash == afterHash {
		fmt.Fprintln(out, "spec unchanged")
		return 0
	}
	fmt.Fprintf(out, "spec updated (%s… → %s…)\n", beforeHash[:8], afterHash[:8])
	fmt.Fprintln(out, "review with `respex diff`, then commit or refine")
	return 0
}
