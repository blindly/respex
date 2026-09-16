package cli

import (
	"flag"
	"fmt"
	"io"
)

var version = "dev"

func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintln(stdout, "respex "+version)
		return 0
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		usage(stdout)
		return 0
	}
	fn, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "respex: unknown command %q\n\n", args[0])
		usage(stderr)
		return 1
	}
	return fn(args[1:], stdout, stderr)
}

var commands = map[string]func(args []string, out, errOut io.Writer) int{
	"new":    runNew,
	"commit": runCommit,
	"diff":   runDiff,
	"apply":  runApply,
	"refine": runRefine,
	"log":    runLog,
	"status":  runStatus,
	"restore": runRestore,
}

func usage(w io.Writer) {
	fmt.Fprint(w, `respex — spec-driven agentic development

Usage:
  respex <command> [args]

Commands:
  new        scaffold .respex/ + SPEC.md (agent-drafted with a description)
  refine     agent critiques and rewrites the spec
  diff       diff working spec vs last commit (or two versions)
  commit     snapshot the working spec as the approved version
  apply      agent makes the codebase match the committed spec
  log        show versions and applies
  status     summarize spec and state
  restore    restore a saved refinement before-state
`)
}

// fail prints an error to the given writer and returns exit code 1.
func fail(w io.Writer, err error) int {
	fmt.Fprintln(w, "respex: "+err.Error())
	return 1
}

// newFlagSet returns a flag set that reports parse errors through errOut.
func newFlagSet(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	return fs
}
