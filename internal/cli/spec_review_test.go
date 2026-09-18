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

func TestSpecReview(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	marker := filepath.Join(root, "marker")
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"-marker\", %q, \"{{prompt}}\"]\n", fakeBin, marker))
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"review"}, &out, &errOut); code != 0 {
		t.Fatalf("spec review = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "review log:") || !strings.Contains(out.String(), "Review the design specification") {
		t.Fatalf("output was not streamed with review log: %s", out.String())
	}
	markerBody, _ := os.ReadFile(marker)
	if !strings.Contains(string(markerBody), "Review the design specification") {
		t.Fatalf("agent did not receive review prompt: %s", markerBody)
	}
}

func TestSpecReviewJSON(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeConfig(t, root, fmt.Sprintf("[agent]\ncommand = [%q, \"{{prompt}}\"]\n", fakeBin))
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"review", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("spec review json = %d, %s | %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"critique"`) {
		t.Fatalf("json output missing critique: %s", out.String())
	}
}

func TestSpecReviewNoAgent(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, spec.Skeleton)
	writeConfig(t, root, "")
	var out, errOut bytes.Buffer
	if code := runSpec([]string{"review"}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "no agent configured") {
		t.Fatalf("spec review no agent = %d, %s | %s", code, out.String(), errOut.String())
	}
}
