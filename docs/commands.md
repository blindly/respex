---
layout: default
title: Commands
nav_order: 4
---

# Command reference

```text
respex <command> [args]
```

## Core workflow

### `respex init [description]`
Initialize Respex in the current repository. With a description and configured
agent, drafts the first spec. Without an agent, writes a skeleton.

```text
respex init "a CLI that converts CSV to JSON"
```

### `respex view`
View the working or historical spec. Uses the configured pager on an interactive
terminal; pipes and redirects receive exact Markdown.

```text
respex view
respex view --version 3
respex view --refine latest --before
respex view --refine 4 --after
respex view --baseline latest --before
respex view --baseline 2 --after
respex view --raw
```

### `respex edit`
Open `SPEC.md` in the configured editor and wait for it to close.

### `respex refine [--force] [--no-progress]`
Run the agent to critique and improve the spec. Rejected as a no-op when the
spec is still the generated skeleton or when the same spec/prompt/agent was
last refined unchanged.

### `respex diff [vA vB] | --refine <id|latest> | --baseline <id|latest>`
Show a unified diff of two committed versions, a refinement, or a baseline.
Multi-file specs are diffed file by file.

### `respex commit [-m msg]`
Snapshot the working spec as the approved version. Warns if the content is
unchanged. Multi-file specs are committed as a bundle.

### `respex apply [--agent tpl] [--no-progress]`
Apply the committed spec to the repository. Always uses an immutable snapshot of
the committed version, so external edits cannot affect the run.

## History and recovery

### `respex log [--json]`
Show versions, applies, baselines, and refinements.

### `respex status [--json]`
Summarize the spec, dirty state, apply state, refinement/baseline history, and
agent configuration. Also reports pending split baseline proposals.

### `respex restore <--refine|--baseline> <id|latest> --before`
Restore the working spec to the before-state of a refinement or baseline.
Creates a reversible history entry.

## Baseline

### `respex baseline [--split] [--intent text] [--merge] [--no-progress]`
Derive an initial spec from an existing repository.

```text
respex baseline --intent "purpose of this project"
respex baseline --split --intent "purpose of this project"
respex baseline --merge
```

After `--split`:

```text
respex diff --baseline latest
respex baseline accept
respex baseline discard
```

## Configuration

### `respex config <subcommand>`

```text
respex config init [--local]
respex config path [--local]
respex config edit [--local]
respex config validate [--local]
respex config show [--json]
```

## Maintenance

### `respex update [--check] [--version v...] [--json]`
Secure self-update from GitHub Releases. Verifies the SHA-256 checksum before
replacing the binary.

### `respex doctor [--json]`
Diagnose configuration, project, tools, state, operation lock, and version.

### `respex spec validate`
Check that the working spec has the required sections: Intent, Scope,
Non-Goals, Requirements, Open Questions.

### `respex completion <bash|zsh|fish|powershell>`
Generate shell completion.
