package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/spec"
)

func TestDoctorProject(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	var out, errOut bytes.Buffer
	if code := runDoctor(nil, &out, &errOut); code != 0 {
		t.Fatalf("doctor = %d, %s | %s", code, out.String(), errOut.String())
	}
	for _, want := range []string{"user config", "project config", "state", "schema v8", "spec files", "operation", "version"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor missing %q in %s", want, out.String())
		}
	}
}

func TestDoctorMissingSpecFile(t *testing.T) {
	setupProject(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	if code := runDoctor(nil, &out, &errOut); code != 1 || !strings.Contains(out.String(), "missing: SPEC.md") {
		t.Fatalf("doctor with missing spec = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestDoctorAgentValidation(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{nope}}\"]\n", fakeBin))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	var out, errOut bytes.Buffer
	if code := runDoctor(nil, &out, &errOut); code != 1 || !strings.Contains(out.String(), "unknown placeholder") {
		t.Fatalf("doctor invalid placeholder = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestDoctorAgentCheck(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	var out, errOut bytes.Buffer
	if code := runDoctor([]string{"--agent-check"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "agent responded to test prompt") {
		t.Fatalf("doctor agent check = %d, %s | %s", code, out.String(), errOut.String())
	}
}
