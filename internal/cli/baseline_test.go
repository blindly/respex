package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/state"
)

func TestBaselineGeneratesAndDiffsSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"# Existing project\\n\", \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"--intent", "preserve compatibility"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "baseline generated") {
		t.Fatalf("baseline = %d, %s | %s", code, out.String(), errOut.String())
	}
	body, err := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if err != nil || string(body) != "# Existing project\n" {
		t.Fatalf("baseline spec = %q, %v", body, err)
	}
	out.Reset()
	errOut.Reset()
	if code := runDiff([]string{"--baseline", "latest"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "+# Existing project") {
		t.Fatalf("baseline diff = %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	baselines, err := st.ListBaselines()
	if err != nil || len(baselines) != 1 || baselines[0].Outcome != "generated" {
		t.Fatalf("baselines = %+v, %v", baselines, err)
	}
}

func TestBaselineRequiresMergeForMeaningfulSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# Existing intent\n")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runBaseline(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "--merge") {
		t.Fatalf("baseline without merge = %d, %s | %s", code, out.String(), errOut.String())
	}
	if code := runBaseline([]string{"--merge"}, &out, &errOut); code != 0 {
		t.Fatalf("baseline merge = %d, %s | %s", code, out.String(), errOut.String())
	}
}
