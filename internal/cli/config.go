package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blindly/respex/internal/config"
)

func runConfig(args []string, out, errOut io.Writer) int {
	if len(args) != 1 || (args[0] != "init" && args[0] != "path") {
		return fail(errOut, fmt.Errorf("usage: respex config <init|path>"))
	}
	path, err := config.GlobalPath()
	if err != nil {
		return fail(errOut, err)
	}
	if args[0] == "path" {
		fmt.Fprintln(out, path)
		return 0
	}
	if _, err := os.Stat(path); err == nil {
		return fail(errOut, fmt.Errorf("user config already exists: %s", path))
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(errOut, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fail(errOut, err)
	}
	if err := os.WriteFile(path, []byte(configTemplate), 0o644); err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "created user config: %s\n", path)
	return 0
}
