package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"respex/internal/state"
)

func TestApplyDirtySpecFails(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	writeSpec(t, root, "# one\nchanged\n")
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "spec changed since last commit") {
		t.Fatalf("apply dirty: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyRequiresAgent(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no agent configured") {
		t.Fatalf("apply without agent: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyNoCommits(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "no committed versions yet") {
		t.Fatalf("apply without commit: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".respex", "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyInvalidAgentFlag(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply([]string{"--agent", `"claude"`}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "invalid --agent template") {
		t.Fatalf("invalid agent flag: %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runApply([]string{"oops"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "unexpected argument") {
		t.Fatalf("positional arg: %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestApplyMissingBinary(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, "[agent]\ncommand = [\"respex-no-such-binary-xyz\", \"{{prompt}}\"]\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "respex-no-such-binary-xyz") {
		t.Fatalf("missing binary: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	unfinished, err := st.HasUnfinishedApply()
	if err != nil {
		t.Fatal(err)
	}
	if unfinished {
		t.Fatal("apply row left unfinished after missing-binary error")
	}
	if _, err := os.Stat(filepath.Join(root, ".respex", "logs", "1-apply.log")); err != nil {
		t.Fatalf("log file: %v", err)
	}
}

func TestApplyLogSetupFailureStampsRow(t *testing.T) {
	root := setupProject(t)
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced on windows")
	}
	writeSpec(t, root, "# one\n")
	writeConfig(t, root, "[agent]\ncommand = [\"respex-no-such-binary-xyz\", \"{{prompt}}\"]\n")
	runCommit(nil, &bytes.Buffer{}, &bytes.Buffer{})
	logsDir := filepath.Join(root, ".respex", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(logsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(logsDir, 0o755) })
	var out, errOut bytes.Buffer
	if code := runApply(nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "permission denied") {
		t.Fatalf("log setup: %d, %s | %s", code, out.String(), errOut.String())
	}
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	unfinished, err := st.HasUnfinishedApply()
	if err != nil {
		t.Fatal(err)
	}
	if unfinished {
		t.Fatal("apply row left unfinished after log setup failure")
	}
}
