---
layout: default
title: Open Questions workflow
nav_order: 8
---

# Open Questions workflow

Open Questions are not defects in the spec. They are **deliberate decision debt**:
things the agent cannot determine from the repository or from a brief.
Resolving them is a human job.

See [Writing specs](writing-specs/) for how to phrase decisions as requirements
once you know the answer.

## When Open Questions appear

- During `respex baseline` of an existing codebase, because observed code may
  contradict itself or depend on external context.
- During manual drafting, when you intentionally leave a decision for later.
- During `respex refine`, if the agent surfaces an ambiguity but cannot choose
  the answer without domain knowledge.

For raw ideas that are not yet ready for an Open Question section, use
`respex notes add` instead of editing the spec.

## The resolution loop

```text
respex notes add "..."      # capture half-formed ideas without touching the spec
respex spec questions       # see every open question in the bundle
respex edit                 # answer the blocking ones
respex diff                 # review what changed
respex commit -m "resolve address, API, and asset open questions"
respex apply                # implement the now-stable spec
```

## How to answer an open question

Edit the spec and turn the question into a durable project decision:

| If the answer is... | Move it to... |
|---|---|
| A behavior the agent must implement | `## Requirements` |
| Something the project will not do | `## Non-Goals` |
| A documented constraint or assumption | `## Scope` or a note under the relevant requirement |
| A decision that depends on future work | Keep it in `## Open Questions` with a note explaining why |

Once a question is answered, remove it from `## Open Questions`. The section
should only contain unresolved items.

## Blockers versus tolerable debt

Some questions block an apply; others do not.

**Blocking** — resolve before apply:

- Which address is authoritative?
- Should `/api/lead` run as a Netlify function?
- What is the supported Node version?

**Tolerable** — can stay open while applying around them:

- Should a future cleanup remove unused components?
- Which assets will be supplied later?
- Should a dead page be restored or deleted?

If the `## Open Questions` section is the largest part of the spec, the project
is not ready to apply. Commit a question-only checkpoint if you want a snapshot.

## Multi-file specs

In a bundle, Open Questions can live in the master spec or in a feature file.
`respex spec questions` collects all of them:

```text
respex spec questions
respex edit auth             # answer auth-specific questions
respex edit                  # answer project-wide questions
respex commit -m "resolve auth and project open questions"
```

## Reviewing what changed

After editing, diff the spec before committing:

```text
respex diff
```

This shows exactly which questions were answered and where they moved.

## Restoring a previous version

If an answer turns out to be wrong, use the refinement/baseline history:

```text
respex log
respex diff --refine latest
respex restore --refine latest --before
```

Restore is reversible, so experimenting with answers is safe.

## JSON output for automation

```text
respex spec questions --json
```

Use this to export the question list to a ticket tracker, a meeting agenda, or a
CI check that fails when the number of blocking open questions is too high.
