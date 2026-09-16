package agent

import "strings"

// Built-in prompt templates. Placeholders {{prompt}} and {{spec_path}} are
// substituted with a plain replacer — NOT Go text/template, whose delimiters
// collide with the placeholder syntax.
const (
	PromptDraft = `Write a design specification document at {{spec_path}} for the idea below.
Use exactly these sections: Intent, Scope, Non-Goals, Requirements, Open Questions.
Write concrete, testable requirements. Do not create or modify any other files.

Idea: {{prompt}}`

	PromptBaseline = `Derive a baseline design specification for the existing repository and write it at {{spec_path}}.
Inspect the repository thoroughly, including documentation, source code, tests, configuration, build files, public interfaces, and platform assumptions.
Describe observable current behavior as concrete requirements. Do not invent intent or non-goals that cannot be established from evidence; put uncertainty in Open Questions.
Use exactly these sections: Intent, Scope, Non-Goals, Requirements, Open Questions.
Do not create or modify any other files.

Additional intent supplied by the user: {{prompt}}`

	PromptRefine = `Refine the design specification at {{spec_path}}.
First read the spec, then inspect this repository to ground the critique in
what actually exists (real file names, real constraints).
Fix ambiguity, contradictions, and gaps. Improve structure. Preserve intent.
Rewrite the spec file in place. Do not modify any other files.
Finish by summarizing the changes you made.`

	PromptApply = `The repository must conform to the design specification at {{spec_path}}.
Read the spec fully before making changes. Make the repository match the spec.
The spec file is immutable: do not modify it.
Finish with a short summary of the changes you made.`
)

const (
	PlaceholderPrompt   = "{{prompt}}"
	PlaceholderSpecPath = "{{spec_path}}"
)

// Expand substitutes placeholders in a prompt template. Replacement values are
// not rescanned: Expand performs a single pass, so prompt text that itself
// mentions template syntax is preserved verbatim.
func Expand(tmpl, prompt, specPath string) string {
	return strings.NewReplacer(PlaceholderSpecPath, specPath, PlaceholderPrompt, prompt).Replace(tmpl)
}
