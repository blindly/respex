// Command fakeagent is a stub agent for tests: it records or writes files and
// exits with a controllable code.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() {
	marker := flag.String("marker", "", "append the received prompt to this file")
	write := flag.String("write", "", "write -content to this file")
	content := flag.String("content", "", "content written by -write")
	bundle := flag.String("bundle", "", "write a split-spec proposal (SPEC.md + specs/*.md) into this directory")
	fail := flag.Bool("fail", false, "exit with code 1")
	sleep := flag.Duration("sleep", 0, "sleep before exiting")
	conforms := flag.String("conforms", "", "print a CONFORMS verdict line with this value (yes|no)")
	flag.Parse()

	prompt, _ := io.ReadAll(os.Stdin)
	if len(prompt) == 0 && flag.NArg() > 0 {
		prompt = []byte(flag.Arg(0))
	}
	fmt.Print(string(prompt))
	if *sleep > 0 {
		time.Sleep(*sleep)
	}
	if *marker != "" {
		f, err := os.OpenFile(*marker, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(2)
		}
		fmt.Fprintf(f, "\n---\n%s", prompt)
		f.Close()
	}
	if *write != "" {
		if err := os.WriteFile(*write, []byte(*content), 0o644); err != nil {
			os.Exit(2)
		}
	}
	if *bundle != "" {
		files := map[string]string{
			"SPEC.md":            "# Split project\n\n## Intent\n\nExisting codebase.\n\n## Scope\n\nAll.\n\n## Non-Goals\n\nNone.\n\n## Requirements\n\n- Works.\n\n## Features\n\n- [Alpha](specs/alpha.md)\n- [Beta](specs/beta.md)\n\n## Open Questions\n\n- None.\n",
			"specs/alpha.md":     "# Alpha\n\n## Intent\n\nAlpha feature.\n\n## Scope\n\nAlpha.\n\n## Non-Goals\n\nNone.\n\n## Requirements\n\n- Alpha works.\n\n## Dependencies\n\nNone.\n\n## Open Questions\n\n- None.\n",
			"specs/beta.md":      "# Beta\n\n## Intent\n\nBeta feature.\n\n## Scope\n\nBeta.\n\n## Non-Goals\n\nNone.\n\n## Requirements\n\n- Beta works.\n\n## Dependencies\n\n- alpha\n\n## Open Questions\n\n- None.\n",
			"notes/internal.txt": "ignored",
		}
		for rel, body := range files {
			dest := filepath.Join(*bundle, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				os.Exit(2)
			}
			if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
				os.Exit(2)
			}
		}
	}
	if *conforms != "" {
		fmt.Printf("CONFORMS: %s\n", *conforms)
	}
	if *fail {
		os.Exit(1)
	}
}
