package agent

import (
	"reflect"
	"strings"
	"testing"
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
