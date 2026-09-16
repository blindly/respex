package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/blindly/respex/internal/spec"
)

func TestAutomationCommandsJSON(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for name, run := range map[string]func(*bytes.Buffer, *bytes.Buffer) int{
		"status": func(out, errOut *bytes.Buffer) int { return runStatus([]string{"--json"}, out, errOut) },
		"log":    func(out, errOut *bytes.Buffer) int { return runLog([]string{"--json"}, out, errOut) },
		"config": func(out, errOut *bytes.Buffer) int { return runConfig([]string{"show", "--json"}, out, errOut) },
		"doctor": func(out, errOut *bytes.Buffer) int { return runDoctor([]string{"--json"}, out, errOut) },
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(&out, &errOut); code != 0 || !json.Valid(out.Bytes()) {
				t.Fatalf("%s JSON = %d, %s | %s", name, code, out.String(), errOut.String())
			}
		})
	}
}
