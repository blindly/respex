// Command fakeagent is a stub agent for tests: it records or writes files and
// exits with a controllable code.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	marker := flag.String("marker", "", "append the received prompt to this file")
	write := flag.String("write", "", "write -content to this file")
	content := flag.String("content", "", "content written by -write")
	fail := flag.Bool("fail", false, "exit with code 1")
	sleep := flag.Duration("sleep", 0, "sleep before exiting")
	flag.Parse()

	prompt, _ := io.ReadAll(os.Stdin)
	if len(prompt) == 0 && flag.NArg() > 0 {
		prompt = []byte(flag.Arg(0))
	}
	fmt.Print(string(prompt))
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
	if *sleep > 0 {
		time.Sleep(*sleep)
	}
	if *fail {
		os.Exit(1)
	}
}
