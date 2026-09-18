package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/blindly/respex/internal/spec"
)

func warnMissingSections(out io.Writer, content []byte) {
	if missing := spec.MissingSections(content); len(missing) > 0 {
		fmt.Fprintf(out, "warning: spec is missing recommended sections: %s\n", strings.Join(missing, ", "))
	}
}

func runSpec(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return fail(errOut, fmt.Errorf("usage: respex spec <validate|questions|review>"))
	}
	switch args[0] {
	case "validate":
		return runSpecValidate(args[1:], out, errOut)
	case "questions":
		return runSpecQuestions(args[1:], out, errOut)
	case "review":
		return runSpecReview(args[1:], out, errOut)
	default:
		return fail(errOut, fmt.Errorf("usage: respex spec <validate|questions|review>"))
	}
}

func runSpecValidate(args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		return fail(errOut, fmt.Errorf("usage: respex spec validate"))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	files, err := w.readSpecFiles()
	if err != nil {
		return fail(errOut, err)
	}
	missing := spec.MissingSections(files[0].Content)
	if len(missing) > 0 {
		return fail(errOut, fmt.Errorf("spec is missing required sections: %s", strings.Join(missing, ", ")))
	}
	if len(files) > 1 {
		fmt.Fprintf(out, "spec bundle is structurally valid (%d files)\n", len(files))
		return 0
	}
	fmt.Fprintln(out, "spec is structurally valid")
	return 0
}
