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
	"time"
)

var respexBin, fakeBin string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Fprintln(os.Stderr, "integration tests need the go toolchain on PATH:", err)
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "respex-integration")
	if err != nil {
		panic(err)
	}
	// os.Exit below skips defers; the explicit RemoveAll covers the normal
	// path and the defer covers a build panic unwinding the stack.
	defer os.RemoveAll(dir)
	respexBin = filepath.Join(dir, "respex")
	fakeBin = filepath.Join(dir, "fakeagent")
	if runtime.GOOS == "windows" {
		respexBin += ".exe"
		fakeBin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", respexBin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building respex failed:\n%s\n", out)
		panic(err)
	}
	if out, err := exec.Command("go", "build", "-o", fakeBin, "./testdata/fakeagent").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building fakeagent failed:\n%s\n", out)
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

	// Refine round-trip: the agent rewrites the spec, which is then
	// committed and applied as v2.
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-write\", %q, \"-content\", \"# Demo\\n\\n## Intent\\n\\nRefined by agent.\\n\", \"-marker\", %q, \"{{prompt}}\"]\n",
			fakeBin, filepath.Join(root, "SPEC.md"), filepath.Join(root, "marker")))
	out, code = run(t, root, "refine")
	if code != 0 || !strings.Contains(out, "spec updated") {
		t.Fatalf("refine: %d, %s", code, out)
	}
	refined, _ := os.ReadFile(filepath.Join(root, "SPEC.md"))
	if !strings.Contains(string(refined), "Refined by agent.") {
		t.Fatalf("refined spec: %q", refined)
	}
	out, code = run(t, root, "commit", "-m", "refined")
	if code != 0 || !strings.Contains(out, "committed v2") {
		t.Fatalf("commit refined: %d, %s", code, out)
	}
	out, code = run(t, root, "apply")
	if code != 0 || !strings.Contains(out, "applied v2") {
		t.Fatalf("apply v2: %d, %s", code, out)
	}
	marker2, _ := os.ReadFile(filepath.Join(root, "marker"))
	if strings.Count(string(marker2), "\n---\n") != 3 {
		t.Fatalf("agent should have run three times (apply, refine, apply), marker:\n%s", marker2)
	}

	// Dirty spec guard.
	write(t, filepath.Join(root, "SPEC.md"), "# Demo\n\n## Intent\n\nDo the thing better.\n")
	out, code = run(t, root, "apply")
	if code != 1 || !strings.Contains(out, "spec changed since last commit") {
		t.Fatalf("dirty guard: %d, %s", code, out)
	}
	marker3, _ := os.ReadFile(filepath.Join(root, "marker"))
	if strings.Count(string(marker3), "\n---\n") != 3 {
		t.Fatalf("dirty-spec apply must not run the agent, marker:\n%s", marker3)
	}
}

func TestApplyFailureThenRetry(t *testing.T) {
	root := isolate(t)
	out, code := run(t, root, "new")
	if code != 0 {
		t.Fatalf("new: %d, %s", code, out)
	}
	write(t, filepath.Join(root, "SPEC.md"), "# x\n")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-fail\", \"{{prompt}}\"]\n", fakeBin))
	out, code = run(t, root, "commit")
	if code != 0 {
		t.Fatalf("commit: %d, %s", code, out)
	}

	out, code = run(t, root, "apply")
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
	out, code := run(t, root, "new")
	if code != 0 {
		t.Fatalf("new: %d, %s", code, out)
	}
	write(t, filepath.Join(root, "SPEC.md"), "# x\n")
	marker := filepath.Join(root, "marker")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q]\ndelivery = \"stdin\"\n", fakeBin, marker))
	out, code = run(t, root, "commit")
	if code != 0 {
		t.Fatalf("commit: %d, %s", code, out)
	}
	out, code = run(t, root, "apply")
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

func TestApplyInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT delivery is not exercised on windows")
	}
	root := isolate(t)
	out, code := run(t, root, "new")
	if code != 0 {
		t.Fatalf("new: %d, %s", code, out)
	}
	write(t, filepath.Join(root, "SPEC.md"), "# x\n")
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"-sleep\", \"5s\", \"{{prompt}}\"]\n", fakeBin))
	out, code = run(t, root, "commit")
	if code != 0 {
		t.Fatalf("commit: %d, %s", code, out)
	}

	// Start apply, then interrupt it mid-run. Wait until the agent has
	// actually begun (its prompt shows up in the apply log), which also
	// guarantees respex has registered its SIGINT handler — a blind sleep
	// can starve under load and kill respex before the handler exists.
	var buf bytes.Buffer
	cmd := exec.Command(respexBin, "apply")
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, ".respex", "logs", "1-apply.log")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if fi, err := os.Stat(logPath); err == nil && fi.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("apply never reached the agent; output:\n%s", buf.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		cmd.Process.Kill()
		t.Fatal(err)
	}
	_ = cmd.Wait()
	code = cmd.ProcessState.ExitCode()
	if code != 1 && code != 130 {
		t.Fatalf("interrupted apply exit: %d, %s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "apply interrupted") {
		t.Fatalf("interrupted apply output:\n%s", buf.String())
	}

	// The interrupted run is recorded and a later apply can proceed.
	write(t, filepath.Join(root, ".respex", "config.toml"),
		fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	out, code = run(t, root, "apply")
	if code != 0 || strings.Contains(out, "warning: a previous apply did not finish") || !strings.Contains(out, "applied v1") {
		t.Fatalf("apply after interrupt: %d, %s", code, out)
	}
}
