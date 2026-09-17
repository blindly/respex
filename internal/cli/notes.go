package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blindly/respex/internal/ui"
)

func runNotes(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return showNotes(out, errOut)
	}
	switch args[0] {
	case "show":
		return showNotes(out, errOut)
	case "add":
		if len(args) < 2 {
			return fail(errOut, errors.New("usage: respex notes add <text>"))
		}
		return addNote(strings.Join(args[1:], " "), out, errOut)
	case "edit":
		if len(args) != 1 {
			return fail(errOut, errors.New("usage: respex notes edit"))
		}
		return editNotes(out, errOut)
	case "clear":
		if len(args) != 1 {
			return fail(errOut, errors.New("usage: respex notes clear"))
		}
		return clearNotes(out, errOut)
	default:
		return fail(errOut, fmt.Errorf("usage: respex notes <show|add|edit|clear>"))
	}
}

func notesWorkspace() (*workspace, error) {
	w, err := discover()
	if err != nil {
		return nil, err
	}
	return w, nil
}

func ensureNotesFile(w *workspace) (string, error) {
	path := w.notesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create notes directory: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte("# Notes\n\nScratchpad for ideas that are not yet part of the spec.\n"), 0o644); err != nil {
			return "", fmt.Errorf("create notes file: %w", err)
		}
	}
	return path, nil
}

func showNotes(out, errOut io.Writer) int {
	w, err := notesWorkspace()
	if err != nil {
		return fail(errOut, err)
	}
	path := w.notesPath()
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "no notes yet")
		return 0
	}
	if err != nil {
		return fail(errOut, err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		fmt.Fprintln(out, "no notes yet")
		return 0
	}
	if !ui.IsTTY(out) {
		_, err = out.Write(content)
		if err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	pager, err := resolvePager(w.cfg.Pager)
	if err != nil {
		return fail(errOut, err)
	}
	if len(pager) == 0 {
		_, err = out.Write(content)
		if err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	cmd := exec.Command(pager[0], pager[1:]...)
	cmd.Stdin = bytes.NewReader(content)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fail(errOut, fmt.Errorf("pager: %w", err))
	}
	return 0
}

func addNote(text string, out, errOut io.Writer) int {
	w, err := notesWorkspace()
	if err != nil {
		return fail(errOut, err)
	}
	path, err := ensureNotesFile(w)
	if err != nil {
		return fail(errOut, err)
	}
	line := fmt.Sprintf("- %s — %s\n", time.Now().UTC().Format("2006-01-02"), text)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fail(errOut, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintln(out, "note added")
	return 0
}

func editNotes(out, errOut io.Writer) int {
	w, err := notesWorkspace()
	if err != nil {
		return fail(errOut, err)
	}
	path, err := ensureNotesFile(w)
	if err != nil {
		return fail(errOut, err)
	}
	editor, err := resolveEditor(w.cfg.Editor)
	if err != nil {
		return fail(errOut, err)
	}
	cmd := exec.Command(editor[0], append(editor[1:], path)...)
	cmd.Dir = w.root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fail(errOut, fmt.Errorf("editor: %w", err))
	}
	fmt.Fprintln(out, "notes updated")
	return 0
}

func clearNotes(out, errOut io.Writer) int {
	w, err := notesWorkspace()
	if err != nil {
		return fail(errOut, err)
	}
	path := w.notesPath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "no notes to clear")
		return 0
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintln(out, "notes cleared")
	return 0
}
