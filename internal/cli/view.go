package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/blindly/respex/internal/spec"
	"github.com/blindly/respex/internal/ui"
)

func resolvePager(configured []string) ([]string, error) {
	if len(configured) > 0 {
		return configured, nil
	}
	if value := os.Getenv("PAGER"); value != "" {
		return splitEditor(value)
	}
	candidates := [][]string{{"less", "-FRX"}, {"more"}}
	if runtime.GOOS == "windows" {
		candidates = [][]string{{"more"}}
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate, nil
		}
	}
	return nil, nil
}

func historyID(value, kind string) (int64, bool, error) {
	if value == "latest" {
		return 0, true, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false, fmt.Errorf("invalid %s id %q; want a positive integer or latest", kind, value)
	}
	return id, false, nil
}

func runView(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("view", errOut)
	raw := fs.Bool("raw", false, "print directly without a pager")
	versionID := fs.Int64("version", 0, "view a committed version")
	refineID := fs.String("refine", "", "view a refinement id or latest")
	baselineID := fs.String("baseline", "", "view a baseline id or latest")
	before := fs.Bool("before", false, "view the before snapshot")
	after := fs.Bool("after", false, "view the after snapshot")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex view [--raw] [--version id | --refine id --before|--after | --baseline id --before|--after]", fs.Arg(0)))
	}
	sources := 0
	for _, selected := range []bool{*versionID != 0, *refineID != "", *baselineID != ""} {
		if selected {
			sources++
		}
	}
	if sources > 1 || ((*before || *after) && *refineID == "" && *baselineID == "") || (*before && *after) {
		return fail(errOut, errors.New("select one version, refinement, or baseline snapshot"))
	}
	if (*refineID != "" || *baselineID != "") && !*before && !*after {
		return fail(errOut, errors.New("refinement and baseline views require --before or --after"))
	}
	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	var content []byte
	if sources == 0 {
		content, err = spec.Read(w.specPath())
	} else {
		st, openErr := w.openState()
		if openErr != nil {
			return fail(errOut, openErr)
		}
		defer st.Close()
		switch {
		case *versionID != 0:
			v, getErr := st.GetVersion(*versionID)
			if getErr != nil {
				err = getErr
			} else {
				// A committed bundle's first file is the master spec.
				content = versionFiles(v.Content, w.cfg.Spec)[0].Content
			}
		case *refineID != "":
			id, latest, parseErr := historyID(*refineID, "refinement")
			if parseErr != nil {
				err = parseErr
				break
			}
			var rContent []byte
			if latest {
				r, getErr := st.LatestRefine()
				if getErr != nil {
					err = getErr
				} else if r == nil {
					err = errors.New("no refinements yet")
				} else if *before {
					rContent = r.BeforeContent
				} else {
					rContent = r.AfterContent
				}
			} else {
				r, getErr := st.GetRefine(id)
				if getErr != nil {
					err = getErr
				} else if *before {
					rContent = r.BeforeContent
				} else {
					rContent = r.AfterContent
				}
			}
			content = rContent
		case *baselineID != "":
			id, latest, parseErr := historyID(*baselineID, "baseline")
			if parseErr != nil {
				err = parseErr
				break
			}
			if latest {
				b, getErr := st.LatestBaseline()
				if getErr != nil {
					err = getErr
				} else if b == nil {
					err = errors.New("no baselines yet")
				} else if *before {
					content = b.BeforeContent
				} else {
					content = b.AfterContent
				}
			} else {
				b, getErr := st.GetBaseline(id)
				if getErr != nil {
					err = getErr
				} else if *before {
					content = b.BeforeContent
				} else {
					content = b.AfterContent
				}
			}
		}
	}
	if err != nil {
		return fail(errOut, err)
	}
	if len(content) == 0 {
		return fail(errOut, errors.New("selected snapshot is empty"))
	}
	if *raw || !ui.IsTTY(out) {
		if _, err := out.Write(content); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	pager, err := resolvePager(w.cfg.Pager)
	if err != nil {
		return fail(errOut, err)
	}
	if len(pager) == 0 {
		_, err = out.Write(content)
		if err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	cmd := exec.Command(pager[0], pager[1:]...)
	cmd.Stdin = bytes.NewReader(content)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fail(errOut, fmt.Errorf("pager: %w", err))
	}
	return 0
}
