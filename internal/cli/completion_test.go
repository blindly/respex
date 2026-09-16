package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := runCompletion([]string{shell}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "baseline") {
				t.Fatalf("completion %s = %d, %s | %s", shell, code, out.String(), errOut.String())
			}
		})
	}
}
