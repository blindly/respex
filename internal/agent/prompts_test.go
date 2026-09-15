package agent

import "testing"

func TestExpandSubstitutesBoth(t *testing.T) {
	got := Expand("Read {{spec_path}} then {{prompt}}", "do it", "/x/SPEC.md")
	want := "Read /x/SPEC.md then do it"
	if got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

func TestExpandNoPlaceholdersIsIdentity(t *testing.T) {
	s := "plain instructions"
	if Expand(s, "p", "q") != s {
		t.Fatal("Expand changed a template without placeholders")
	}
}
