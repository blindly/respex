package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "respex-cli-test")
	if err != nil {
		panic(err)
	}
	fakeBin = filepath.Join(dir, "fakeagent")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", fakeBin, "../../testdata/fakeagent")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
