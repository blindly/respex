package cli

import (
	"flag"
	"fmt"
	"io"
	"time"

	"respex/internal/spec"
)

func runCommit(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	msg := fs.String("m", "", "commit message")
	fs.Parse(args)

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
