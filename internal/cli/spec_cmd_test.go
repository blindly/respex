package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/blindly/respex/internal/spec"
)

func TestSpecValidate(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"validate"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "structurally valid") {
		t.Fatalf("spec validate = %d, %s | %s", code, out.String(), errOut.String())
	}
	writeSpec(t, root, "## Intent\n")
	out.Reset()
	errOut.Reset()
	if code := runSpec([]string{"validate"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "Scope") {
		t.Fatalf("invalid spec = %d, %s | %s", code, out.String(), errOut.String())
	}
}
