package agent

import "testing"

func TestExpandSubstitutesBoth(t *testing.T) {
	got := Expand("Read {{spec_path}} then {{prompt}}", "do it", "/x/SPEC.md")
	want := "Read /x/SPEC.md then do it"
	if got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

// Expand must not rescan replacement values: a two-pass implementation would
// corrupt prompt text that itself mentions template syntax.
func TestExpandDoesNotRescanReplacements(t *testing.T) {
	got := Expand("At {{spec_path}}: {{prompt}}", "handle {{spec_path}} templating", "/s")
	want := "At /s: handle {{spec_path}} templating"
	if got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

func TestExpandRepeatedPlaceholders(t *testing.T) {
	got := Expand("{{prompt}} then {{prompt}}", "go", "/s")
	want := "go then go"
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
