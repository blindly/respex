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
Initialize ReSpex in the current repository. With a description and configured
agent, drafts the first spec. Without an agent, writes a skeleton.

```text
respex init "a CLI that converts CSV to JSON"
```

### `respex view [feature]`
View the working or historical spec. In a multi-file project, pass a feature
name (the basename without `.md`) to view one feature spec. Uses the configured
pager on an interactive terminal; pipes and redirects receive exact Markdown.

```text
respex view
respex view auth
respex view --version 3
respex view --refine latest --before
respex view --refine 4 --after
respex view --baseline latest --before
respex view --baseline 2 --after
respex view --raw
```

### `respex edit [feature]`
Open `SPEC.md` or the named feature spec in the configured editor and wait for
it to close.

### `respex refine [--force] [--no-progress] [--notes] [feature]`
Run the agent to critique and improve the spec. In a multi-file project, pass a
feature name to refine only that feature spec; the agent reads the master spec
for context but must not change it. Rejected as a no-op when the spec is still the
generated skeleton or when the same spec/prompt/agent was last refined unchanged.
Use `--notes` to also pass the project notes file as context.

### `respex diff [vA vB] | --refine <id|latest> | --baseline <id|latest>`
Show a unified diff of two committed versions, a refinement, or a baseline.
Multi-file specs are diffed file by file.

### `respex commit [-m msg]`
Snapshot the working spec as the approved version. Warns if the content is
unchanged. Multi-file specs are committed as a bundle.

### `respex apply [--agent tpl] [--no-progress] [feature]`
Apply the committed spec to the repository. Pass a feature name to implement
only that feature from the committed bundle. Always uses an immutable snapshot of
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

### `respex notes <show|add|edit|clear>`
A project scratchpad for ideas that are not yet part of the spec.

```text
respex notes
respex notes add "explore dark mode"
respex notes edit
respex notes clear
```

Notes live at `.respex/notes.md` by default (set `notes` in config to change the
path). They are not committed with the spec and are ignored by `respex check`.
Use `respex refine --notes` to let the agent read them as context.

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

### `respex doctor [--json] [--agent-check]`
Diagnose configuration, project, tools, state, operation lock, and version.
The `agent` check validates that the configured agent binary exists and that
its command template uses supported placeholders correctly. Add `--agent-check`
to also run a short test prompt through the agent and verify it responds.

### `respex spec validate`
Check that the working spec has the required sections: Intent, Scope,
Non-Goals, Requirements, Open Questions.

### `respex spec questions [--json]`
List all open questions from the master spec and any configured feature specs.
Useful for reviewing what still needs a human decision before applying.

### `respex spec review [--notes] [--json] [feature]`
Ask the configured agent to review the spec and repository, then report issues
without modifying any files. The agent reads the spec, linked feature specs, the
codebase, tests, and configuration, and outputs a critique with Critical,
Warnings, and Suggestions sections. Add `--notes` to also pass the project
scratchpad as context. Agent output streams to the terminal by default while
also being captured in `.respex/logs/`. JSON mode remains silent until it emits
the final JSON document.

### `respex check [--json]`
Validate the configured spec bundle for structural and quality problems: missing
spec files, empty files, missing required sections, broken internal Markdown
links, unlinked feature files, non-prescriptive requirement language
(e.g. "fix the broken..."), and `TODO`/`FIXME` markers. Exits non-zero when any
structural check fails; quality issues are reported as warnings.

### `respex completion <bash|zsh|fish|powershell>`
Generate shell completion.
