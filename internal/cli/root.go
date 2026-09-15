package cli

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

func Main(args []string) int {
	if len(args) > 0 && (args[0] == "--version" || args[0] == "version") {
		fmt.Println("respex " + version)
		return 0
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		usage(os.Stdout)
		return 0
	}
	fn, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "respex: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 1
	}
	return fn(args[1:], os.Stdout)
}

// commands is filled in by each command task.
var commands = map[string]func(args []string, out io.Writer) int{}

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
`)
}

// fail prints an error to stderr and returns exit code 1.
func fail(err error) int {
	fmt.Fprintln(os.Stderr, "respex: "+err.Error())
	return 1
}