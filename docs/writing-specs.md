---
layout: default
title: Writing specs
nav_order: 7
---

# Writing specs

The spec is a **contract** for the codebase. Write it as the intended end state,
not as a list of observations or bugs.

## Prescriptive, not descriptive

When you find a problem, do not describe the problem in the spec. State the rule
the code must follow once the agent is done.

| Instead of | Write |
|---|---|
| The `/terms` page shows the wrong address. | The canonical address is **123 Example Lane, Sampletown, ST 12345** and every page, component, and structured-data field must use it. |
| The NAP data is inconsistent. | All NAP data must be consistent across pages, SEO metadata, and structured data. |
| The build uses Node 18 locally. | The project targets Node 18 for CI and production builds. |

The agent reads the spec as instructions. If the spec only says "something is
wrong," the agent has no instruction to follow.

## Sections and their purpose

A typical spec uses these sections:

- **Intent** — why the project exists.
- **Scope** — what is included now.
- **Non-Goals** — what is explicitly out of scope.
- **Requirements** — the rules the code must satisfy.
- **Open Questions** — genuine decisions that have not been made yet.
- **Notes** — raw ideas that are not yet questions or requirements.

## Open Questions, Notes, and Requirements

These three containers look similar but behave differently:

| Container | Use for | Example |
|---|---|---|
| `## Notes` (`.respex/notes.md`) | Half-formed ideas you are not ready to decide. | "Maybe add dark mode." |
| `## Open Questions` | Decisions the project needs but you cannot make yet. | "Which address is authoritative?" |
| `## Requirements` | Decisions you have made and want the agent to enforce. | "The canonical address is 123 Example Lane." |

Once you know the answer to an Open Question, move it into Requirements and remove
it from Open Questions.

## Turning an Open Question into a requirement

Starting state:

```markdown
## Open Questions

- Which NAP data is authoritative? The contact page says 123 Example Lane, while
  `/terms` says 456 Sample Road.
```

After deciding:

```markdown
## Requirements

- The canonical address is **123 Example Lane, Sampletown, ST 12345**.
- Every page, component, SEO metadata block, and structured-data field must use
  the canonical address.
- Any page that currently shows a different address must be reconciled.

## Open Questions

- (none, or only unresolved items)
```

Now `respex apply` has a clear instruction set.

## The edit/commit/apply loop

1. Edit the spec to state the intended end state.
2. Review the diff.
3. Commit the spec.
4. Apply it to the code.
5. Review the code changes.

```text
respex edit
respex diff
respex commit -m "require canonical NAP data and consistent hours"
respex apply
respex diff
```

If you edit the spec and then run `respex refine`, be aware that refine is an
agent rewrite. It may reword or remove your edits if it thinks they are unclear
or contradictory. Commit the spec first so you can restore your manual version if
needed.

## Do not use the spec as a bug tracker

The spec should not accumulate tickets like this:

```markdown
- Fix the `/terms` address.
- Remove dead ContactForm imports.
- Update Node in CI.
```

Those are tasks, not requirements. Turn them into requirements:

```markdown
- The `/terms` page must display the canonical address.
- The `ContactForm` component must not be imported unless it is used.
- CI must run on Node 18.
```

If you need a task list, keep it in your project notes or issue tracker. The spec
should describe the world as it should be.

## When the agent removes something you added

If you add a section and a later `respex refine` removes it, the agent probably
did one of these:

- It did not understand the section as a requirement.
- It found the section contradicted another part of the spec.
- It considered the heading non-standard and folded it elsewhere.

To protect the content, phrase it as a numbered requirement under `##
Requirements`, use clear imperatives, and commit before refining.

## Automatic checks

`respex check` now also flags common spec-quality mistakes:

- `TODO`/`FIXME` markers in the spec.
- Requirement bullets that read like bug-tracker entries (`fix the broken...`,
  `currently shows...`, `inconsistent`, `missing`, etc.).
- Missing required sections and broken internal links.

Run it after editing and before applying:

```text
respex edit
respex check
respex commit -m "specify canonical NAP data"
respex apply
```

If you want to enforce that no blocking Open Questions exist before a CI run,
you can check programmatically:

```text
respex spec questions --json
respex check --json
```

Use these in a CI gate so a spec with unresolved blockers cannot be applied
automatically.
