package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/blindly/respex/internal/spec"
)

func runCommit(args []string, out, errOut io.Writer) int {
	fs := newFlagSet("commit", errOut)
	msg := fs.String("m", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return fail(errOut, err)
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("unexpected argument %q — usage: respex commit [-m msg]", fs.Arg(0)))
	}

	w, err := discover()
	if err != nil {
		return fail(errOut, err)
	}
	content, err := spec.Read(w.specPath())
	if err != nil {
		return fail(errOut, err)
	}
	st, err := w.openState()
	if err != nil {
		return fail(errOut, err)
	}
	defer st.Close()
	h := spec.Hash(content)
	if last, err := st.LatestVersion(); err != nil {
		return fail(errOut, err)
	} else if last != nil && last.Hash == h {
		fmt.Fprintln(out, "warning: spec content is unchanged; committing anyway")
	}
	id, err := st.InsertVersion(h, content, *msg, time.Now())
	if err != nil {
		return fail(errOut, err)
	}
	fmt.Fprintf(out, "committed v%d (%s…)\n", id, h[:8])
	return 0
}
