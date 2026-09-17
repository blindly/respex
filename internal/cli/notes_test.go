package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotesAddAndShow(t *testing.T) {
	root := setupProject(t)
	writeConfig(t, root, "") // ensure project config exists
	var out, errOut bytes.Buffer
	if code := runNotes([]string{"add", "explore dark mode"}, &out, &errOut); code != 0 {
		t.Fatalf("notes add = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "note added") {
		t.Fatalf("missing note added: %s", out.String())
	}
	out.Reset()
	if code := runNotes(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "explore dark mode") {
		t.Fatalf("notes show = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestNotesClear(t *testing.T) {
	root := setupProject(t)
	writeConfig(t, root, "")
	if code := runNotes([]string{"add", "idea one"}, ioDiscard(), ioDiscard()); code != 0 {
		t.Fatal("add failed")
	}
	var out, errOut bytes.Buffer
	if code := runNotes([]string{"clear"}, &out, &errOut); code != 0 {
		t.Fatalf("notes clear = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := runNotes(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "no notes yet") {
		t.Fatalf("notes show after clear = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestNotesEdit(t *testing.T) {
	root := setupProject(t)
	writeConfig(t, root, fmt.Sprintf("editor = [%q, \"-write\", %q, \"-content\", \"# Notes\\n\\n- edited\\n\"]\n", fakeBin, filepath.Join(root, ".respex", "notes.md")))
	var out, errOut bytes.Buffer
	if code := runNotes([]string{"edit"}, &out, &errOut); code != 0 {
		t.Fatalf("notes edit = %d, %s | %s", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(filepath.Join(root, ".respex", "notes.md"))
	if !strings.Contains(string(body), "- edited") {
		t.Fatalf("notes body = %s", body)
	}
}

func TestRefineWithNotes(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n## Open Questions\n")
	marker := filepath.Join(root, "marker")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"# refined\\n\", \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	if err := os.MkdirAll(filepath.Join(root, ".respex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".respex", "notes.md"), []byte("- consider dark mode\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runRefine([]string{"--notes"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "spec updated") {
		t.Fatalf("refine --notes = %d, %s | %s", code, out.String(), errOut.String())
	}
	markerBody, _ := os.ReadFile(marker)
	if !strings.Contains(string(markerBody), ".respex/notes.md") {
		t.Fatalf("refine prompt did not reference notes: %s", markerBody)
	}
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }
