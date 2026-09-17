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

	PromptBaselineSplit = `Derive a baseline design specification for the existing repository and write it as multiple Markdown files inside the directory {{spec_path}}.
Inspect the repository thoroughly, including documentation, source code, tests, configuration, build files, public interfaces, and platform assumptions.
Write {{spec_path}}/SPEC.md as the master specification with exactly these sections: Intent, Scope, Non-Goals, Requirements, Features, Open Questions. The Features section must link to each feature file with a relative Markdown link.
Write each durable product capability as {{spec_path}}/specs/<feature>.md using a lowercase hyphenated name, with these sections: Intent, Scope, Non-Goals, Requirements, Dependencies, Open Questions.
Organize around stable product capabilities, not source directories. Create at most 8 feature files.
Describe observable current behavior as concrete requirements. Do not invent intent or non-goals that cannot be established from evidence; put uncertainty in Open Questions.
Do not create or modify any files outside {{spec_path}}.

Additional intent supplied by the user: {{prompt}}`

	PromptRefine = `Refine the design specification at {{spec_path}}.
First read the spec, then inspect this repository to ground the critique in
what actually exists (real file names, real constraints).
Fix ambiguity, contradictions, and gaps. Improve structure. Preserve intent.
Rewrite the spec file in place. Do not modify any other files.
Finish by summarizing the changes you made.`

	PromptRefineFeature = `Refine the feature specification at {{spec_path}}.
First read this feature spec, then read the master specification for project-wide
context, then inspect this repository to ground the critique in what actually exists.
Fix ambiguity, contradictions, and gaps within this feature. Improve structure.
Preserve intent. Keep the feature spec focused on this capability and its
boundaries; do not duplicate content that belongs in the master spec.
Rewrite only the feature spec file at {{spec_path}}. Do not modify any other files,
including the master specification.
Finish by summarizing the changes you made to this feature.`

	PromptApply = `The repository must conform to the design specification at {{spec_path}}.
Read the spec fully before making changes. If the spec links to other
specification files alongside it, read them too — they are part of the spec.
Make the repository match the spec.
The spec files are immutable: do not modify them.
Finish with a short summary of the changes you made.`

	PromptApplyFeature = `The repository must implement the feature described in the feature specification at {{spec_path}}.
First read the master specification at {{master_spec_path}} for project-wide context,
then read this feature specification thoroughly.
Make only the changes needed to implement this feature in the repository.
Do not modify any specification files.
Finish with a short summary of the changes you made.`
)

const (
	PlaceholderPrompt         = "{{prompt}}"
	PlaceholderSpecPath       = "{{spec_path}}"
	PlaceholderMasterSpecPath = "{{master_spec_path}}"
)

// Expand substitutes placeholders in a prompt template. Replacement values are
// not rescanned: Expand performs a single pass, so prompt text that itself
// mentions template syntax is preserved verbatim. The optional masterSpecPath
// is used when refining or applying an individual feature spec.
func Expand(tmpl, prompt, specPath string, masterSpecPath ...string) string {
	repl := []string{PlaceholderSpecPath, specPath, PlaceholderPrompt, prompt}
	if len(masterSpecPath) > 0 {
		repl = append(repl, PlaceholderMasterSpecPath, masterSpecPath[0])
	}
	return strings.NewReplacer(repl...).Replace(tmpl)
}
