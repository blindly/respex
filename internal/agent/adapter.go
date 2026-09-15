package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	DeliveryArgv  = "argv"
	DeliveryStdin = "stdin"

	// maxArgvBytes keeps the substituted command line under the Windows
	// CreateProcess limit.
	maxArgvBytes = 32 * 1024
)

var placeholderRe = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// checkPlaceholders rejects unknown placeholders before expansion.
func checkPlaceholders(tpl []string) error {
	for _, el := range tpl {
		for _, m := range placeholderRe.FindAllString(el, -1) {
			if m != PlaceholderPrompt && m != PlaceholderSpecPath {
				return fmt.Errorf("agent: unknown placeholder %s; supported: {{prompt}}, {{spec_path}}", m)
			}
		}
	}
	return nil
}

// Build validates the template and returns the prepared command. In stdin
// delivery the returned byte slice carries the prompt; in argv delivery it is nil.
func Build(tpl []string, delivery, prompt, specPath string) (*exec.Cmd, []byte, error) {
	if len(tpl) == 0 {
		return nil, nil, errors.New("agent: empty command template")
	}
	if tpl[0] == "" {
		return nil, nil, errors.New("agent: command element 0 (binary) is empty — set [agent] command in .respex/config.toml")
	}
	if err := checkPlaceholders(tpl); err != nil {
		return nil, nil, err
	}
	joined := strings.Join(tpl, " ")
	switch delivery {
	case DeliveryArgv:
		if !strings.Contains(joined, PlaceholderPrompt) && !strings.Contains(joined, PlaceholderSpecPath) {
			return nil, nil, fmt.Errorf(
				"agent: delivery %q requires {{prompt}} or {{spec_path}} in the command", DeliveryArgv)
		}
	case DeliveryStdin:
		if strings.Contains(joined, PlaceholderPrompt) {
			return nil, nil, fmt.Errorf(
				`agent: delivery %q forbids {{prompt}} in the command; remove it and pipe instead`, DeliveryStdin)
		}
	default:
		return nil, nil, fmt.Errorf("agent: unknown delivery %q; want %q or %q", delivery, DeliveryArgv, DeliveryStdin)
	}
	argv := make([]string, len(tpl))
	total := 0
	for i, el := range tpl {
		argv[i] = Expand(el, prompt, specPath)
		total += len(argv[i]) + 1
	}
	// Size guard against the Windows CreateProcess command-line limit. The
	// count is in bytes, which is conservative versus UTF-16 units (bytes are
	// never fewer); Windows quoting overhead (syscall.EscapeArg) is not
	// modeled.
	if delivery == DeliveryArgv && total > maxArgvBytes {
		return nil, nil, fmt.Errorf(
			"agent: command line is %d bytes (limit %d) — set delivery = \"stdin\"", total, maxArgvBytes)
	}
	var stdin []byte
	if delivery == DeliveryStdin {
		stdin = []byte(prompt)
	}
	return exec.Command(argv[0], argv[1:]...), stdin, nil
}

// Adapter is a resolved agent command: argv template + delivery mode + extras.
type Adapter struct {
	Command  []string
	Delivery string
	Env      []string
	Dir      string
}

// Execute runs the adapter with the given instruction text. Agent stdout and
// stderr tee to the terminal and to out. Returns the exit code; a canceled
// context returns (-1, context.Canceled).
func (a Adapter) Execute(ctx context.Context, prompt, specPath string, out io.Writer) (int, error) {
	cmd, stdin, err := Build(a.Command, a.Delivery, prompt, specPath)
	if err != nil {
		return -1, err
	}
	cmd.Dir = a.Dir
	cmd.Env = append(os.Environ(), a.Env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Stdout = io.MultiWriter(os.Stdout, out)
	cmd.Stderr = io.MultiWriter(os.Stderr, out)
	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("agent: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return -1, fmt.Errorf("agent: %w", err)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return -1, ctx.Err()
	}
}
