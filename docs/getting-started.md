---
layout: default
title: Getting started
nav_order: 2
---

# Getting started

## Initialize a new project

```text
respex init "a CLI that converts CSV to JSON"
```

This creates:

```text
.respex/
├── config.toml
├── state.db
├── logs/
└── tmp/
SPEC.md
```

If an agent is configured, `respex init "..."` drafts the first `SPEC.md` from
the description. Otherwise it writes a skeleton and tells you to configure an
agent and run `respex refine`.

## Existing repository

For an existing codebase, derive an initial spec from observed code, tests,
documentation, and configuration:

```text
respex init
respex baseline --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex commit -m "baseline existing implementation"
```

Use `respex baseline --merge` when `SPEC.md` already contains meaningful
content.

Use `respex baseline --split` for large codebases that need a multi-file spec
bundle. See [Multi-file specs](multi-file-specs/).

## Edit and refine

```text
respex edit              # open SPEC.md in your configured editor
respex refine            # agent improves the spec using the repository
respex diff --refine latest
respex commit -m "refined auth requirements"
```

Refinement is repeatable. ReSpex stores the before/after content of every
refinement so you can review or restore.

## Apply the spec

```text
respex apply
```

Apply gives the agent an immutable copy of the committed spec, then runs the
agent against the repository. Output is captured in `.respex/logs/`.

Run `respex apply` again and ReSpex reports that the version is already applied.
