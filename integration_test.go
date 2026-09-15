package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var respexBin, fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "respex-integration")
	if err != nil {
		panic(err)
	}
	respexBin = filepath.Join(dir, "respex")
	fakeBin = filepath.Join(dir, "fakeagent")
	if runtime.GOOS == "windows" {
		respexBin += ".exe"
		fakeBin += ".exe"
	}
	if err := exec.Command("go", "build", "-o", respexBin, ".").Run(); err != nil {
		panic(err)
	}
	if err := exec.Command("go", "build", "-o", fakeBin, "./testdata/fakeagent").Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func run(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	cmd := exec.Command(respexBin, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return buf.String(), code
}

func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("APPDATA", cfg)
	return root
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleEndToEnd(t *testing.T) {
	root := isolate(t)

	out, code := run(t, root, "new")
	if code != 0 || !strings.Contains(out, "created SPEC.md") {
		t.Fatalf("new: %d, %s", code, out)
	}
	write(t, filepath.Join(root, "SPEC.md"), "# Demo\n\n## Intent\n\nDo the thing.\n")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, filepath.Join(root, "marker")))

	out, code = run(t, root, "commit", "-m", "first")
	if code != 0 || !strings.Contains(out, "committed v1") {
		t.Fatalf("commit: %d, %s", code, out)
	}

	out, code = run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "applied v1") {
		t.Fatalf("apply: %d, %s", code, out)
	}
	marker, _ := os.ReadFile(filepath.Join(root, "marker"))
	if !strings.Contains(string(marker), "conform") || !strings.Contains(string(marker), "SPEC.md") {
		t.Fatalf("apply prompt wrong: %q", marker)
	}

	out, code = run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "nothing to do (v1 already applied)") {
		t.Fatalf("no-op apply: %d, %s", code, out)
	}

	// Dirty spec guard.
	write(t, filepath.Join(root, "SPEC.md"), "# Demo\n\n## Intent\n\nDo the thing better.\n")
	out, code = run(t, root, "apply")
	if code != 1 || !strings.Contains(out, "spec changed since last commit") {
		t.Fatalf("dirty guard: %d, %s", code, out)
	}

	// Commit v2 and re-apply: the new version applies.
	out, code = run(t, root, "commit", "-m", "second")
	if code != 0 || !strings.Contains(out, "committed v2") {
		t.Fatalf("commit2: %d, %s", code, out)
	}
	out, code = run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "applied v2") {
		t.Fatalf("apply v2: %d, %s", code, out)
	}
	marker2, _ := os.ReadFile(filepath.Join(root, "marker"))
	if strings.Count(string(marker2), "\n---\n") != 2 {
		t.Fatalf("agent should have run twice, marker:\n%s", marker2)
	}
}

func TestApplyFailureThenRetry(t *testing.T) {
	root := isolate(t)
	run(t, root, "new")
	write(t, filepath.Join(root, "SPEC.md"), "# x\n")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-fail\", \"{{prompt}}\"]\n", fakeBin))
	run(t, root, "commit")

	out, code := run(t, root, "apply")
	if code != 1 || !strings.Contains(out, "failed (exit 1)") {
		t.Fatalf("failing apply: %d, %s", code, out)
	}

	// Retry with a succeeding agent: failed applies do not count as applied.
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	out, code = run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "applied v1") {
		t.Fatalf("retry after failure: %d, %s", code, out)
	}
}

func TestStdinDelivery(t *testing.T) {
	root := isolate(t)
	run(t, root, "new")
	write(t, filepath.Join(root, "SPEC.md"), "# x\n")
	marker := filepath.Join(root, "marker")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q]\ndelivery = \"stdin\"\n", fakeBin, marker))
	run(t, root, "commit")
	out, code := run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "applied v1") {
		t.Fatalf("stdin apply: %d, %s", code, out)
	}
	marker2, _ := os.ReadFile(filepath.Join(root, "marker"))
	if !strings.Contains(string(marker2), "conform") {
		t.Fatalf("prompt not delivered via stdin:\n%s", marker2)
	}
}

func TestNewDraftViaAgent(t *testing.T) {
	root := isolate(t)
	// Pre-seed the user-level config so `new` can draft through the agent.
	gcDir := filepath.Join(root, "cfg", "respex")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(gcDir, "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", \"{{spec_path}}\", \"-content\", \"drafted spec\"]\n", fakeBin))
	out, code := run(t, root, "new", "a tool that sorts files")
	if code != 0 || !strings.Contains(out, "drafted SPEC.md") {
		t.Fatalf("draft new: %d, %s", code, out)
	}
	body, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if string(body) != "drafted spec" {
		t.Fatalf("SPEC.md = %q", body)
	}
}
