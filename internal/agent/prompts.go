package agent

import "strings"

// Built-in prompt templates. Placeholders {{prompt}} and {{spec_path}} are
// substituted with a plain replacer — NOT Go text/template, whose delimiters
// collide with the placeholder syntax.
const (
	PromptDraft = `Write a design specification document at {{spec_path}} for the idea below.
Use exactly these sections: Intent, Scope, Non-Goals, Requirements, Open Questions.
Write concrete, testable requirements. Do not create any other files.

Idea: {{prompt}}`

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

// Expand substitutes placeholders in a prompt template.
func Expand(tmpl, prompt, specPath string) string {
	return strings.NewReplacer(PlaceholderSpecPath, specPath, PlaceholderPrompt, prompt).Replace(tmpl)
}
