package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/state"
)

func setupBundleProject(t *testing.T) string {
	root := setupProject(t)
	writeSpec(t, root, "# Product\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n## Features\n\n- [Auth](specs/auth.md)\n\n## Open Questions\n")
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "specs", "auth.md"), []byte("# Auth\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n## Open Questions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, root, fmt.Sprintf("spec_files = [\"specs/auth.md\"]\n[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	return root
}

func TestResolveFeature(t *testing.T) {
	_ = setupBundleProject(t)
	w, err := discover()
	if err != nil {
		t.Fatal(err)
	}
	m, err := w.featureMap()
	if err != nil {
		t.Fatal(err)
	}
	if m["auth"] != "specs/auth.md" {
		t.Fatalf("feature map = %+v", m)
	}
	if _, ok := m["SPEC"]; ok {
		t.Fatal("master spec should not appear in feature map")
	}
}

func TestViewFeature(t *testing.T) {
	_ = setupBundleProject(t)
	var out, errOut bytes.Buffer
	if code := runView([]string{"auth"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "# Auth") {
		t.Fatalf("view auth = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineFeature(t *testing.T) {
	root := setupBundleProject(t)
	marker := filepath.Join(root, "marker")
	writeConfig(t, root, fmt.Sprintf("spec_files = [\"specs/auth.md\"]\n[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"# Auth refined\\n\", \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	var out, errOut bytes.Buffer
	if code := runRefine([]string{"auth"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "spec updated") {
		t.Fatalf("refine auth = %d, %s | %s", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(filepath.Join(root, "specs", "auth.md"))
	if string(body) != "# Auth refined\n" {
		t.Fatalf("auth.md = %q", body)
	}
	master, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if !strings.Contains(string(master), "# Product") {
		t.Fatalf("master spec was modified: %s", master)
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	refines, err := st.ListRefines()
	if err != nil || len(refines) != 1 || refines[0].TargetPath != "specs/auth.md" {
		t.Fatalf("refinements = %+v, %v", refines, err)
	}
}

func TestApplyFeature(t *testing.T) {
	root := setupBundleProject(t)
	writeConfig(t, root, fmt.Sprintf("spec_files = [\"specs/auth.md\"]\n[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	if code := runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("commit = %d", code)
	}
	var out, errOut bytes.Buffer
	if code := runApply([]string{"auth"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "applied feature auth") {
		t.Fatalf("apply auth = %d, %s | %s", code, out.String(), errOut.String())
	}
	var again bytes.Buffer
	if code := runApply([]string{"auth"}, &again, &errOut); code != 0 || !strings.Contains(again.String(), "already applied") {
		t.Fatalf("apply auth again = %d, %s", code, again.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	applied, err := st.IsFeatureApplied(1, "specs/auth.md")
	if err != nil || !applied {
		t.Fatalf("feature applied = %v, %v", applied, err)
	}
}

func TestCheckBundle(t *testing.T) {
	_ = setupBundleProject(t)
	var out, errOut bytes.Buffer
	if code := runCheck(nil, &out, &errOut); code != 0 {
		t.Fatalf("check = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "feature-index: specs/auth.md linked") {
		t.Fatalf("check output missing feature-index pass: %s", out.String())
	}
}

func TestCheckBrokenLink(t *testing.T) {
	root := setupBundleProject(t)
	// Break the feature link in the master spec.
	writeSpec(t, root, "# Product\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n## Features\n\n- [Auth](specs/missing.md)\n\n## Open Questions\n")
	var out, errOut bytes.Buffer
	if code := runCheck(nil, &out, &errOut); code != 1 {
		t.Fatalf("check broken link = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "missing spec") {
		t.Fatalf("check output missing broken link failure: %s", out.String())
	}
}

func TestCheckBugTrackerLanguage(t *testing.T) {
	root := setupBundleProject(t)
	writeSpec(t, root, "# Product\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n- Fix the broken login button.\n- The terms page currently shows the wrong address.\n\n## Open Questions\n")
	var out, errOut bytes.Buffer
	if code := runCheck(nil, &out, &errOut); code != 0 {
		t.Fatalf("check bug language = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "bug-tracker") {
		t.Fatalf("check did not warn about bug-tracker language: %s", out.String())
	}
}

func TestCheckTodoMarker(t *testing.T) {
	root := setupBundleProject(t)
	writeSpec(t, root, "# Product\n\n## Intent\n\n## Scope\n\n## Non-Goals\n\n## Requirements\n\n- TODO: define payment flow.\n\n## Open Questions\n")
	var out, errOut bytes.Buffer
	if code := runCheck(nil, &out, &errOut); code != 1 {
		t.Fatalf("check todo = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "TODO/FIXME") {
		t.Fatalf("check did not fail on TODO marker: %s", out.String())
	}
}
