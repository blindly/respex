---
layout: default
title: Multi-file specs
nav_order: 6
---

# Multi-file specs

Repositories with multiple durable product capabilities can represent the
specification as a bundle: a master spec plus one file per capability. Repository
size alone is not a reason to split.

```text
SPEC.md
specs/authentication.md
specs/billing.md
specs/reporting.md
```

`SPEC.md` stays the master document: project-wide intent, scope, non-goals,
system invariants, a feature index linking to each feature file, and open
questions. Each `specs/<feature>.md` describes one capability with its own
intent, scope, requirements, dependencies, and questions.

## Configure spec_files

```toml
spec = "SPEC.md"
spec_files = ["specs/authentication.md", "specs/billing.md", "specs/reporting.md"]
```

Project-level `spec_files` overrides the user-level setting. Once configured,
`respex` treats the whole bundle as a single specification version.

## Generate a multi-file baseline

```text
respex init
respex baseline --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex baseline accept
respex commit -m "multi-file baseline"
```

`baseline` prefers one specification and chooses multiple files only for durable
product capabilities with independent requirement boundaries. A bundle is written
inside `.respex/proposals/baseline-*/`; the live repository is unchanged while the
proposal is pending. `baseline accept` installs the files, updates
`.respex/config.toml`, and records the row as accepted. `baseline discard`
removes the proposal. The deprecated `--split` flag is a compatibility no-op.

## Validation and limits

A multi-file proposal must contain:

- `SPEC.md` (or whatever `spec` is configured to)
- At least one and at most 8 files matching `specs/<feature>.md`
- Lowercase-hyphenated feature names

Unexpected files in the proposal directory are ignored with a warning.

## Bundle behavior

When `spec_files` is configured:

- `commit` snapshots the whole bundle atomically.
- `diff` compares the bundle file by file.
- `status` and `doctor` track every configured file.
- `apply` hands the agent an immutable copy of the entire committed bundle.
- `apply auth` or `refine auth` targets a single feature spec while using the
  master spec as context.
- `view auth` and `edit auth` open an individual feature spec.
- `respex check` validates the whole bundle for broken links, missing files, and
  missing required sections.
- `restore` currently operates on the master spec.
