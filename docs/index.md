---
layout: default
title: Respex
nav_order: 1
---

# Respex

Spec-driven agentic development: one markdown spec per repo, applied by any agent CLI.

`respex` manages the lifecycle of a design spec and dispatches any agentic CLI
(Claude Code, Gemini CLI, Codex, aider, opencode, amp, …) to act on it:

```text
init → view/edit → refine → diff → commit → apply
```

The spec is the durable source of truth; the codebase converges to it. Respex
coordinates commits, agent runs, history, and rollback so changes are
reviewable and recoverable.

## Install

### GitHub Releases

Download the binary for your operating system and architecture from
[GitHub Releases](../../releases), then place it on your `PATH`.

Prebuilt binaries are available for Linux, macOS, and Windows on amd64 and arm64.

### Build from source

Clone the repository and run:

```text
go build .
```

## Quick start

```text
respex init "a CLI that converts CSV to JSON"
respex view
respex edit
respex refine
respex diff --refine latest
respex diff
respex commit -m "initial spec"
respex apply
respex apply          # → "nothing to do (v1 already applied)"
```

## Next steps

- [Getting started](getting-started/) — initialize, baseline, and iterate.
- [End-to-end process](end-to-end/) — the complete lifecycle and when to use each command.
- [Configuration](configuration/) — user and project settings.
- [Commands](commands/) — full command reference.
- [Workflows](workflows/) — `refine`, `baseline`, `apply`, and rollback patterns.
- [Multi-file specs](multi-file-specs/) — splitting large specifications into a bundle.
