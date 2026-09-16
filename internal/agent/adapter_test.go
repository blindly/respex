package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildSubstitutesArgv(t *testing.T) {
	cmd, stdin, err := Build([]string{"agent", "-p", "{{prompt}}", "{{spec_path}}"}, DeliveryArgv, "do it", "/r/S.md")
	if err != nil {
		t.Fatal(err)
	}
	if stdin != nil {
		t.Fatal("argv delivery must not set stdin")
	}
	want := []string{"agent", "-p", "do it", "/r/S.md"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
}

func TestBuildStdinDelivery(t *testing.T) {
	cmd, stdin, err := Build([]string{"agent", "-p"}, DeliveryStdin, "do it", "/r/S.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(stdin) != "do it" {
		t.Fatalf("stdin = %q", stdin)
	}
	if strings.Join(cmd.Args, " ") != "agent -p" {
		t.Fatalf("args = %v", cmd.Args)
	}
}

func TestBuildStdinForbidsPromptPlaceholder(t *testing.T) {
	_, _, err := Build([]string{"agent", "{{prompt}}"}, DeliveryStdin, "p", "s")
	if err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildArgvRequiresPlaceholder(t *testing.T) {
	_, _, err := Build([]string{"agent", "-p"}, DeliveryArgv, "p", "s")
	if err == nil || !strings.Contains(err.Error(), "{{prompt}}") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildUnknownPlaceholder(t *testing.T) {
	_, _, err := Build([]string{"agent", "{{nope}}", "{{prompt}}"}, DeliveryArgv, "p", "s")
	if err == nil || !strings.Contains(err.Error(), "{{nope}}") {
		t.Fatalf("err = %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "supported") {
		t.Fatalf("err must list supported placeholders, got %v", err)
	}
}

func TestBuildUnknownDelivery(t *testing.T) {
	_, _, err := Build([]string{"agent", "{{prompt}}"}, "Argv", "p", "s")
	if err == nil || !strings.Contains(err.Error(), "unknown delivery") {
		t.Fatalf("err = %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), `want "argv" or "stdin"`) {
		t.Fatalf("err must name valid deliveries, got %v", err)
	}
}

func TestBuildEmptyBinary(t *testing.T) {
	_, _, err := Build([]string{""}, DeliveryArgv, "p", "s")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty binary must be rejected, got %v", err)
	}
}

func TestBuildStdinSpecPath(t *testing.T) {
	cmd, stdin, err := Build([]string{"agent", "{{spec_path}}"}, DeliveryStdin, "p", "/r/S.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(stdin) != "p" {
		t.Fatalf("stdin = %q", stdin)
	}
	if strings.Join(cmd.Args, " ") != "agent /r/S.md" {
		t.Fatalf("args = %v", cmd.Args)
	}
}

func TestBuildSizeLimit(t *testing.T) {
	big := strings.Repeat("x", 40_000)
	_, _, err := Build([]string{"agent", "-p", "{{prompt}}"}, DeliveryArgv, big, "s")
	if err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Fatalf("oversize argv must suggest stdin, got %v", err)
	}
	// Same prompt through stdin is fine.
	if _, _, err := Build([]string{"agent"}, DeliveryStdin, big, "s"); err != nil {
		t.Fatal(err)
	}
}

var fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "respex-agent-test")
	if err != nil {
		panic(err)
	}
	fakeBin = filepath.Join(dir, "fakeagent")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", fakeBin, "../../testdata/fakeagent")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestExecuteSuccessCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	log := &bytes.Buffer{}
	a := Adapter{
		Command:  []string{fakeBin, "-marker", marker, "{{prompt}}"},
		Delivery: DeliveryArgv,
		Dir:      dir,
	}
	code, err := a.Execute(context.Background(), "hello agent", "s", log)
	if err != nil || code != 0 {
		t.Fatalf("Execute = %d, %v", code, err)
	}
	got, _ := os.ReadFile(marker)
	if !strings.Contains(string(got), "hello agent") {
		t.Fatalf("marker = %q", got)
	}
	if !strings.Contains(log.String(), "hello agent") {
		t.Fatalf("captured output missing prompt: %q", log.String())
	}
}

func TestExecuteFailureExitCode(t *testing.T) {
	a := Adapter{Command: []string{fakeBin, "-fail", "{{prompt}}"}, Delivery: DeliveryArgv, Dir: t.TempDir()}
	code, err := a.Execute(context.Background(), "p", "s", io.Discard)
	if err != nil || code != 1 {
		t.Fatalf("Execute = %d, %v; want 1, nil", code, err)
	}
}

func TestExecuteMissingBinary(t *testing.T) {
	a := Adapter{Command: []string{"respex-no-such-binary-xyz", "{{prompt}}"}, Delivery: DeliveryArgv, Dir: t.TempDir()}
	code, err := a.Execute(context.Background(), "p", "s", io.Discard)
	if err == nil || code != -1 {
		t.Fatalf("Execute = %d, %v; want -1, error", code, err)
	}
}

func TestExecuteInterrupt(t *testing.T) {
	dir := t.TempDir()
	wrote := filepath.Join(dir, "after-kill")
	a := Adapter{Command: []string{fakeBin, "-sleep", "3s", "-write", wrote, "{{prompt}}"}, Delivery: DeliveryArgv, Dir: dir}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	code, err := a.Execute(ctx, "p", "s", io.Discard)
	if !errors.Is(err, context.Canceled) || code != -1 {
		t.Fatalf("Execute = %d, %v; want -1, context.Canceled", code, err)
	}
	if _, err := os.Stat(wrote); !os.IsNotExist(err) {
		t.Fatalf("killed agent must not have written %s", wrote)
	}
}
