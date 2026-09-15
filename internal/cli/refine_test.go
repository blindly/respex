package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
