package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/spec"
)

func writeSplitConfig(t *testing.T, root string) {
	t.Helper()
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-bundle\", \"{{spec_path}}\", \"{{prompt}}\"]\n", fakeBin))
}

func TestBaselineSplitProposeAcceptCommit(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeSplitConfig(t, root)

	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"--split"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "split baseline proposed (3 files)") {
		t.Fatalf("baseline --split = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "ignoring unexpected proposal file notes/internal.txt") {
		t.Fatalf("expected ignored-file warning, out = %s", out.String())
	}

	// The live spec must be untouched while the proposal is pending.
	body, err := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if err != nil || string(body) != spec.Skeleton {
		t.Fatalf("live spec changed before accept: %q, %v", body, err)
	}

	// A second baseline is refused while the proposal is pending.
	out.Reset()
	errOut.Reset()
	if code := runBaseline(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "pending") {
		t.Fatalf("baseline during pending = %d, %s | %s", code, out.String(), errOut.String())
	}

	// Review shows every proposed file.
	out.Reset()
	errOut.Reset()
	if code := runDiff([]string{"--baseline", "latest"}, &out, &errOut); code != 0 {
		t.Fatalf("diff --baseline = %d, %s", code, errOut.String())
	}
	for _, want := range []string{"SPEC.md", "specs/alpha.md", "specs/beta.md", "+# Alpha", "+# Beta"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("split diff missing %q:\n%s", want, out.String())
		}
	}

	// Accept installs the bundle and configures spec_files.
	out.Reset()
	errOut.Reset()
	if code := runBaseline([]string{"accept"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "accepted split baseline #1") {
		t.Fatalf("baseline accept = %d, %s | %s", code, out.String(), errOut.String())
	}
	for _, rel := range []string{"specs/alpha.md", "specs/beta.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("installed file %s: %v", rel, err)
		}
	}
	if body, err := os.ReadFile(filepath.Join(root, "SPEC.md")); err != nil || !strings.Contains(string(body), "# Split project") {
		t.Fatalf("master spec = %q, %v", body, err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, ".respex", "config.toml"))
	if err != nil || !strings.Contains(string(cfg), `spec_files = ["specs/alpha.md", "specs/beta.md"]`) {
		t.Fatalf("config.toml = %q, %v", cfg, err)
	}

	// Commit snapshots the whole bundle; status stays clean.
	out.Reset()
	errOut.Reset()
	if code := runCommit(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "committed v1") || !strings.Contains(out.String(), "3 spec files") {
		t.Fatalf("commit = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runStatus(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "dirty:       no") || !strings.Contains(out.String(), "spec files:  3") {
		t.Fatalf("status = %d, %s | %s", code, out.String(), errOut.String())
	}

	// Editing a feature spec dirties the bundle and diffs per file.
	if err := os.WriteFile(filepath.Join(root, "specs", "alpha.md"), []byte("# Alpha\n\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := runDiff(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "v1:specs/alpha.md") || !strings.Contains(out.String(), "+changed") {
		t.Fatalf("bundle diff = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runStatus(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "dirty:       yes") {
		t.Fatalf("dirty status = %d, %s", code, out.String())
	}
}

func TestBaselineSplitDiscard(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeSplitConfig(t, root)
	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"--split"}, &out, &errOut); code != 0 {
		t.Fatalf("baseline --split = %d, %s", code, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runBaseline([]string{"discard"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "discarded split baseline proposal #1") {
		t.Fatalf("baseline discard = %d, %s | %s", code, out.String(), errOut.String())
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".respex", "proposals")); err != nil || len(entries) != 0 {
		t.Fatalf("proposal dir not removed: %v, %v", entries, err)
	}
	// A fresh proposal can be generated after discarding.
	out.Reset()
	errOut.Reset()
	if code := runBaseline([]string{"--split"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "proposed") {
		t.Fatalf("re-split after discard = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestBaselineAcceptRefusesChangedSpec(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeSplitConfig(t, root)
	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"--split"}, &out, &errOut); code != 0 {
		t.Fatalf("baseline --split = %d, %s", code, errOut.String())
	}
	writeSpec(t, root, "# user edits\n")
	out.Reset()
	errOut.Reset()
	if code := runBaseline([]string{"accept"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "changed since the proposal") {
		t.Fatalf("accept after edit = %d, %s | %s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "alpha.md")); !os.IsNotExist(err) {
		t.Fatal("feature files must not be installed on refused accept")
	}
}

func TestBaselineAcceptWithoutProposal(t *testing.T) {
	setupProject(t)
	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"accept"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no pending split baseline proposal") {
		t.Fatalf("accept without proposal = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyBundleSnapshot(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeSplitConfig(t, root)
	var out, errOut bytes.Buffer
	if code := runBaseline([]string{"--split"}, &out, &errOut); code != 0 {
		t.Fatalf("baseline --split = %d, %s", code, errOut.String())
	}
	if code := runBaseline([]string{"accept"}, &out, &errOut); code != 0 {
		t.Fatalf("baseline accept = %d, %s", code, errOut.String())
	}
	if code := runCommit(nil, &out, &errOut); code != 0 {
		t.Fatalf("commit = %d, %s", code, errOut.String())
	}
	// Apply with a marker agent; the prompt must reference a snapshot path
	// under .respex/tmp whose sibling spec files exist.
	marker := filepath.Join(root, "marker.txt")
	writeConfig(t, root, fmt.Sprintf("spec_files = [\"specs/alpha.md\", \"specs/beta.md\"]\n[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	out.Reset()
	errOut.Reset()
	if code := runApply(nil, &out, &errOut); code != 0 || !strings.Contains(out.String(), "applied v1") {
		t.Fatalf("apply bundle = %d, %s | %s", code, out.String(), errOut.String())
	}
	prompt, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), filepath.Join(".respex", "tmp", "apply-v1-")) {
		t.Fatalf("apply prompt missing snapshot path: %s", prompt)
	}
}
