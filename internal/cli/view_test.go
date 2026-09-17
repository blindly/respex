package cli

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindly/respex/internal/state"
)

func TestViewWorkingAndHistory(t *testing.T) {
	root := setupProject(t)
	writeSpec(t, root, "working\n")
	st, err := state.Open(filepath.Join(root, ".respex", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := st.InsertVersion("hash", []byte("version\n"), "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertRefine("agent", "updated", "a", "b", []byte("refine before\n"), []byte("refine after\n"), "", "fingerprint", "", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertBaseline("agent", "generated", "a", "b", []byte("baseline before\n"), []byte("baseline after\n"), nil, "", now, now); err != nil {
		t.Fatal(err)
	}
	st.Close()
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"working":  {nil, "working\n"},
		"version":  {[]string{"--version", "1"}, "version\n"},
		"refine":   {[]string{"--refine", "latest", "--before"}, "refine before\n"},
		"baseline": {[]string{"--baseline", "1", "--after"}, "baseline after\n"},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := runView(tc.args, &out, &errOut); code != 0 || out.String() != tc.want {
				t.Fatalf("view %s = %d, %q | %s", name, code, out.String(), errOut.String())
			}
		})
	}
}

func TestResolvePager(t *testing.T) {
	t.Setenv("PAGER", `"my pager" --quit`)
	pager, err := resolvePager(nil)
	if err != nil || len(pager) != 2 || pager[0] != "my pager" || pager[1] != "--quit" {
		t.Fatalf("pager = %v, %v", pager, err)
	}
	pager, err = resolvePager([]string{"less", "-FRX"})
	if err != nil || len(pager) != 2 || pager[0] != "less" {
		t.Fatalf("configured pager = %v, %v", pager, err)
	}
}
