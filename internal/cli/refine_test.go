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

func TestRefineRewritesSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# draft\n")
	marker := filepath.Join(root, "marker")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"# refined\\n\", \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "spec updated") {
		t.Fatalf("refine = %d, %s | %s", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if string(body) != "# refined\n" {
		t.Fatalf("spec = %q", body)
	}
	markerBody, _ := os.ReadFile(marker)
	if !strings.Contains(string(markerBody), "inspect this repository") {
		t.Fatalf("marker missing PromptRefine text: %q", markerBody)
	}
	if !strings.Contains(string(markerBody), filepath.Join(root, "SPEC.md")) {
		t.Fatalf("marker missing spec path: %q", markerBody)
	}
}

func TestRefineHistoryIsRepeatable(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# same\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	for range 2 {
		if code := runRefine(nil, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("refine exit = %d", code)
		}
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	refines, err := st.ListRefines()
	if err != nil || len(refines) != 2 {
		t.Fatalf("refinements = %+v, %v", refines, err)
	}
	if refines[0].Outcome != "unchanged" || refines[1].Outcome != "unchanged" {
		t.Fatalf("refinement outcomes = %+v", refines)
	}
	if string(refines[0].BeforeContent) != "# same\n" || string(refines[0].AfterContent) != "# same\n" {
		t.Fatalf("refinement snapshots = %+v", refines[0])
	}
	var out, errOut bytes.Buffer
	if code := runStatus(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "refined:     2 times") {
		t.Fatalf("status after refinements = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineUnchangedReports(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# same\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "spec unchanged") {
		t.Fatalf("refine unchanged: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineRejectsConcurrentOperation(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	lock, locked, err := tryApplyLock(filepath.Join(root, ".respex", "operation.lock"))
	if err != nil || !locked {
		t.Fatalf("lock = %v, %v", locked, err)
	}
	defer lock.Close()
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "already running") {
		t.Fatalf("concurrent refine: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineNoAgent(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no agent configured") {
		t.Fatalf("refine no agent: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineRejectsArgs(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	var out, errOut bytes.Buffer
	if code := runRefine([]string{"extra"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "unexpected argument") {
		t.Fatalf("refine args: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineCustomPrompt(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	marker := filepath.Join(root, "marker")
	writeConfig(t, root, fmt.Sprintf("[prompts]\nrefine = \"CUSTOM {{spec_path}}\"\n[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 0 {
		t.Fatalf("refine custom = %d, %s | %s", code, out.String(), errOut.String())
	}
	body, _ := os.ReadFile(marker)
	if !strings.Contains(string(body), "CUSTOM "+filepath.Join(root, "SPEC.md")) {
		t.Fatalf("marker prompt wrong: %q", body)
	}
}

func TestRefineTimeout(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	writeConfig(t, root, fmt.Sprintf("agent_timeout = \"20ms\"\n[agent]\ncommand = [%q, \"-sleep\", \"5s\", \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "refine timed out after 20ms") {
		t.Fatalf("refine timeout: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineAgentFails(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-fail\", \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "refine failed (exit 1)") {
		t.Fatalf("refine fail: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestRefineAgentRemovesSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# x\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runRefine(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "agent removed the spec") {
		t.Fatalf("refine removed: %d, %s | %s", code, out.String(), errOut.String())
	}
}
