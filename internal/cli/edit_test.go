package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEditorPrecedence(t *testing.T) {
	t.Setenv("VISUAL", "visual-editor")
	t.Setenv("EDITOR", "fallback-editor")
	editor, err := resolveEditor([]string{"configured-editor", "--wait"})
	if err != nil || strings.Join(editor, " ") != "configured-editor --wait" {
		t.Fatalf("configured editor = %v, %v", editor, err)
	}
	editor, err = resolveEditor(nil)
	if err != nil || len(editor) != 1 || editor[0] != "visual-editor" {
		t.Fatalf("visual editor = %v, %v", editor, err)
	}
	t.Setenv("VISUAL", "")
	editor, err = resolveEditor(nil)
	if err != nil || len(editor) != 1 || editor[0] != "fallback-editor" {
		t.Fatalf("fallback editor = %v, %v", editor, err)
	}
}

func TestEditUpdatesConfiguredSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# before\n")
	writeConfig(t, root, fmt.Sprintf("editor = [%q, \"-write\", %q, \"-content\", \"# after\\n\"]\n", fakeBin, filepath.Join(root, "SPEC.md")))
	var out, errOut bytes.Buffer
	if code := runEdit(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "spec updated") {
		t.Fatalf("edit = %d, %s | %s", code, out.String(), errOut.String())
	}
	after, err := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if err != nil || string(after) != "# after\n" {
		t.Fatalf("edited spec = %q, %v", after, err)
	}
}

func TestEditRejectsConcurrentOperation(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	lock, locked, err := tryApplyLock(filepath.Join(root, ".respex", "operation.lock"))
	if err != nil || !locked {
		t.Fatalf("lock = %v, %v", locked, err)
	}
	defer lock.Close()
	var out, errOut bytes.Buffer
	if code := runEdit(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "already running") {
		t.Fatalf("concurrent edit = %d, %s | %s", code, out.String(), errOut.String())
	}
}
