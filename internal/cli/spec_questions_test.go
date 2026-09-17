package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecQuestionsMasterOnly(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Open Questions\n\n1. Who is the user?\n2. What is the budget?\n")
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"questions"}, &out, &errOut); code != 0 {
		t.Fatalf("spec questions = %d, %s | %s", code, out.String(), errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "Who is the user?") || !strings.Contains(s, "What is the budget?") {
		t.Fatalf("output missing questions: %s", s)
	}
	if !strings.Contains(s, "2 open question") {
		t.Fatalf("output missing count: %s", s)
	}
}

func TestSpecQuestionsMultiLine(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Open Questions\n\n- **Address** — conflicting addresses in footer and /terms.\n  Which one is authoritative?\n")
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"questions"}, &out, &errOut); code != 0 {
		t.Fatalf("spec questions = %d, %s | %s", code, out.String(), errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "Address") || !strings.Contains(s, "authoritative") {
		t.Fatalf("output missing multiline question: %s", s)
	}
}

func TestSpecQuestionsBundle(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Open Questions\n\n- master question\n")
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "specs", "auth.md"), []byte("# Auth\n\n## Open Questions\n\n- auth question\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, fmt.Sprintf("spec_files = [\"specs/auth.md\"]\n"))
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"questions"}, &out, &errOut); code != 0 {
		t.Fatalf("spec questions = %d, %s | %s", code, out.String(), errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "master question") || !strings.Contains(s, "auth question") {
		t.Fatalf("output missing bundle questions: %s", s)
	}
}

func TestSpecQuestionsJSON(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Open Questions\n\n- one\n")
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"questions", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("spec questions json = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"text": "one"`) {
		t.Fatalf("json output missing question: %s", out.String())
	}
}
