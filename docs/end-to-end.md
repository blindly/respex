---
layout: default
title: End-to-end process
nav_order: 7
---

# End-to-end process

ReSpex is a small state machine around a Markdown spec. The goal is to keep the
spec as the durable source of truth and use an agent CLI to converge the code
toward it.

```text
        ┌─────────────┐
        │  working    │
        │    spec     │
        └──────┬──────┘
               │
      ┌────────┼────────┐
      ▼        ▼        ▼
   edit     refine    baseline
      │        │        │
      └────────┴────────┘
               │
               ▼
        ┌─────────────┐
        │   commit    │  ← snapshot approved version
        └──────┬──────┘
               │
               ▼
        ┌─────────────┐
        │    apply    │  ← agent implements the snapshot
        └──────┬──────┘
               │
               ▼
        ┌─────────────┐
        │  code + git │
        │   commit    │
        └─────────────┘
```

## New project

### 1. Initialize and draft the spec

```text
respex init "a CLI that converts CSV to JSON"
```

If an agent is configured, `init` writes a first `SPEC.md` for you. Otherwise it
writes a skeleton and you should edit by hand or run `respex refine` after
configuring an agent.

### 2. Review and commit

Read the spec, fix anything that is wrong, then snapshot it:

```text
respex view
respex edit
respex commit -m "initial spec"
```

`commit` is the “approve” step. Until you commit, `apply` will refuse to run.

### 3. Optionally refine

Use `refine` when you want the agent to improve the spec for you:

```text
respex commit -m "my manual edits"    # snapshot before the agent touches it
respex refine
respex diff --refine latest
```

`refine` rewrites the spec. It can remove or reword your edits. That is why you
should commit first: the previous version is saved and you can restore it.

```text
respex restore --refine latest --before
```

If the rewrite looks good, commit it:

```text
respex commit -m "refined requirements"
```

### 4. Apply the spec

```text
respex apply
```

Apply gives the agent an immutable copy of the committed spec. The agent edits
the repository, not the spec. Review the code changes with `git diff`, then
commit them in git.

### 5. Iterate

When requirements change, update the spec and repeat:

```text
respex edit
respex commit -m "add JSON output option"
respex apply
git diff
```

`respex apply` skips work that is already applied.

## Open Questions

If your spec has an `## Open Questions` section, resolve the blocking ones
before applying. See the [Open Questions workflow](open-questions/) page for the
full pattern.

## Multi-file projects

When `spec_files` is configured, you can work on one capability at a time:

```text
respex view auth
respex edit auth
respex commit -m "clarify auth requirements"
respex apply auth
respex refine auth
respex check
```

`commit` always snapshots the whole bundle atomically; `apply auth` and
`refine auth` use the master spec as context while touching only that feature.

## Existing project

### 1. Initialize and baseline

```text
cd existing-project
respex init
respex baseline --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex commit -m "baseline existing code"
```

`baseline` inspects the current code, tests, docs, and config, then writes a
spec from observed behavior. It does not invent intent; uncertain areas go into
**Open Questions**.

If `SPEC.md` already has meaningful content, add `--merge`:

```text
respex baseline --merge --intent "..."
```

For larger codebases, use `--split` to propose a multi-file spec bundle:

```text
respex baseline --split --intent "..."
respex diff --baseline latest
respex baseline accept
respex commit -m "split baseline"
```

### 2. Refine or edit, then apply

From this point the workflow is the same as a new project:

```text
respex edit
respex commit
respex apply
```

## When to edit, when to refine

| You want to... | Use |
|---|---|
| Write exactly what you mean | `respex edit` |
| Ask the agent to improve the spec | `respex refine` |
| Derive a spec from existing code | `respex baseline` |
| Snapshot the working spec | `respex commit` |
| Make the repo match the committed spec | `respex apply` |

A safe rule:

- Edit when you know the change you want.
- Refine when you want the agent to critique and rewrite.
- Always commit before a risky refine so `diff` and `restore` work.

## Recovery checkpoints

Every commit and every agent operation that changes the spec saves a
before/after snapshot in `.respex/state.db`.

```text
respex log
respex diff --refine latest
respex diff --baseline latest
respex restore --refine 3 --before
respex restore --baseline 2 --before
```

Restore is itself reversible, so experimenting is safe.

## Inspection commands

```text
respex status          # spec state, dirty flag, operation lock, agent
respex log             # versions, baselines, refinements, applies
respex spec validate   # required sections check
respex doctor          # config, tools, state, lock health
```

## Typical session

```text
respex init "CSV to JSON CLI"
respex view
respex edit
respex commit -m "initial spec"
respex apply
git diff
git commit -m "implement CSV to JSON CLI"

respex edit
respex commit -m "add streaming mode"
respex apply
git diff
git commit -m "add streaming mode"
```
