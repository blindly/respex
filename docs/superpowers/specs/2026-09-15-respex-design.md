# respex — Design

- **Date:** 2026-09-15
- **Status:** Approved (pending implementation plan)
- **Approach:** Thin orchestrator (Option A from brainstorming)

## 1. Summary

`respex` is a single Go binary, cross-platform, hosted on GitHub. It manages the
lifecycle of one markdown specification file per repository and dispatches any
agentic CLI tool to act on it. The spec is the durable source of truth; the
codebase converges to it via `apply`. `respex` owns only lifecycle bookkeeping
and agent dispatch — file editing, repo access, and sandboxing belong to the
agent CLI. `respex` never prompts, never runs shell interpreters, and never
commits code.

## 2. Goals and Non-Goals

### Goals

1. Spec lifecycle: create, refine (agent-assisted), inspect diffs, commit
   versions, apply to the codebase, and audit history.
2. Agent-agnostic integration through a command-template adapter defined as
   configuration data; "any agentic tool" is structural, not a curated list.
3. Deterministic no-op behavior: `apply` runs the agent only when an
   unapplied committed spec version exists. When the opt-in `[verify]` block
   is configured, a version also counts as applied only while its latest
   verification passed.
4. Local, file-only state (SQLite); works in non-git repositories.
5. Cross-platform static binaries (linux/darwin/windows × amd64/arm64).

### Non-Goals (v1)

- No spec trees, multiple specs, or structured spec DSL — one plain markdown file.
- No git-worktree isolation or sandboxing of agents (may layer on later).
- No dashboard, server, or daemon.
- No built-in agent presets at v1 (the template contract suffices; presets are
  config snippets to be added incrementally).
- No git commits of code — the user reviews and commits agent output
  themselves. (Post-apply conformance verification later became an opt-in
  `respex verify` command composed into `apply` via the `[verify]` config
  block and a `verifications` table in the state database, with an optional
  agent audit (`[verify] audit`) reusing the same prompt-template adapter;
  feature-level verification remains out of scope.)
- No remote sync of the state database.

## 3. Lifecycle and Commands

```
respex new [description]   scaffold project + spec; optionally agent-drafts the spec
respex refine              agent critiques and rewrites the spec in place
respex diff [vA vB]        unified diff of spec versions
respex commit [-m msg]     snapshot the working spec as the approved version
respex apply [--agent tpl] agent makes the codebase match the committed spec
respex verify              run configured conformance checks (commands and
                           optional agent audit) against the tree
respex log                 list spec versions, commits, and applies
respex status              summarize spec and state
respex --version | --help
```

### Repository discovery

Commands walk upward from the working directory until a `.respex/` directory is
found (git-style); none found → error "not a respex project — run `respex new`".
`refine` requires only configuration (still needs `.respex/`). The repository
root is the directory containing `.respex/`; all relative paths in config
resolve against it, and the agent runs with it as its working directory.

### `new`

Refuses if `.respex/state.db` already exists or the spec file exists.

Creates:

1. `.respex/config.toml` — `[agent] command = []` left empty with a comment
   showing a filled example.
2. `.respex/state.db` — empty database at current schema version.
3. `SPEC.md` (or configured spec path) — skeleton below.
4. Appends `.respex/` to `.gitignore` when the directory is a git repo and the
   entry is absent.

With a `description` argument *and* a resolvable agent command (project config
or global config), `new` instead asks the agent to draft `SPEC.md` from the
description ("draft" prompt). Without an agent configured, it writes the
skeleton and prints a hint to set `[agent] command` and run `respex refine`.

Skeleton:

```markdown
# <Project title>

## Intent

<What this is and why it exists.>

## Scope

<What is included.>

## Non-Goals

<What is explicitly out of scope.>

## Requirements

- <Behavioral requirement.>

## Open Questions

- <Unresolved decisions.>
```

### `refine`

Runs the agent with the refine prompt (Section 6) against the working spec.
No state is written. On completion prints the before/after content hashes.
Refine on a spec that the agent leaves unchanged is simply a no-op run —
`respex` reports "spec unchanged".

### `diff`

- No args: unified diff of the working spec against the last committed version.
  No committed versions yet → error.
- `vA vB` (integer version ids): diff between two stored snapshots.
- Color added when stdout is a TTY.

### `commit`

Stores a full snapshot row (`hash`, `content`, `committed_at`, optional
`message`) and prints `v<N>`. Committing an identical spec again is allowed
but warns that content is unchanged. There is no branch/merge concept — the
latest commit is always the live version.

### `apply`

Guards, in order:

1. Working spec file must hash equal to the last committed snapshot —
   otherwise exit 1: "spec changed since last commit — review with
   `respex diff`, then `respex commit`".
2. If the last committed version already has a **successful** apply (an
   `applies` row with `exit_code = 0` for that `version_id`): print
   `nothing to do (v<N> already applied)` and exit 0. Failed or interrupted
   applies do not count, so re-running after a failure retries.

Then: build the apply prompt, resolve the adapter argv, insert the apply row
(its id names the log file), execute the agent with output tee'd
(Section 7), and print a summary (version, agent, duration, log path) plus a
`git diff` review hint (git repos only).

`--agent` accepts a one-off command template in TOML array form, e.g.
`--agent '["gemini", "-p", "{{prompt}}"]'`.

### `log`

Prints, newest first: versions (`v<N>`, hash prefix, date, message) and
applies (version, agent, exit code, date). Plain aligned text.

### `status`

Prints: spec path; "dirty" (working ≠ last committed) yes/no; last committed
version; whether that version is applied; agent configured yes/no.

## 4. State — `.respex/state.db`

SQLite through `modernc.org/sqlite` (pure Go, no cgo — keeps
`CGO_ENABLED=0` cross-compilation trivial). Full snapshots, never diff
chains: reconstructing a version is a single row read.

```sql
CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);  -- holds schema_version

CREATE TABLE spec_versions (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,  -- displayed as vN
  hash         TEXT NOT NULL,                      -- sha256 hex of spec bytes
  content      BLOB NOT NULL,                      -- full snapshot
  committed_at TEXT NOT NULL,                      -- RFC 3339 UTC
  message      TEXT
);

CREATE TABLE applies (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  version_id INTEGER NOT NULL REFERENCES spec_versions(id),
  agent      TEXT NOT NULL,        -- resolved command's first token
  started_at TEXT NOT NULL,
  finished_at TEXT,                -- NULL = interrupted
  exit_code  INTEGER,              -- NULL = interrupted
  log_path   TEXT NOT NULL
);

CREATE INDEX idx_applies_version ON applies(version_id);
```

- Schema versioning: `meta.schema_version = 1`; a `state` package migration
  step applies upgrades in order. Schema version 1 is the only one initially.
- History is user data: no command ever deletes rows or the database. Corrupt
  DB → error message suggesting manual recovery.
- The database is local-only and gitignored; a fresh clone starts with no
  history — the spec file itself remains the portable artifact.

## 5. Configuration

Discovery (highest precedence last): global `~/.config/respex/config.toml`
(`%AppData%\respex\config.toml` on Windows) merged under project
`.respex/config.toml`. Top-level keys from the project file win; within
`[agent]` and `[prompts]`, keys merge individually; arrays replace wholesale.

```toml
spec = "SPEC.md"                 # spec file path, relative to repo root

[agent]
command = ["claude", "-p", "{{prompt}}"]  # argv array; placeholders substituted per element
delivery = "argv"                # "argv" | "stdin"; stdin pipes the prompt, {{prompt}} then absent
env = ["KEY=VAL"]                # optional; appended to the inherited environment

[prompts]                        # optional overrides; placeholders {{prompt}}, {{spec_path}}
draft  = "..."
refine = "..."
apply  = "..."
```

Rules:

- `command` MUST be a TOML array of strings — never a shell string. Element 0
  is the binary, resolved via `PATH`. No shell, no quoting/escaping semantics.
- `argv` delivery requires `{{prompt}}` (or `{{spec_path}}`) to appear; if the
  substituted argv would exceed 32 KiB (Windows CreateProcess limit), `respex`
  errors and suggests `delivery = "stdin"`.
- `stdin` delivery requires the array to contain no `{{prompt}}`.
- Empty or missing `[agent] command` → every agent-running command errors with
  setup guidance.
- The config file is data, not a program: no interpolation of other config
  values, no includes.

## 6. Prompt Contract

Built-in prompts (Go text/template over placeholders; overridable via
`[prompts]`). Placeholders: `{{prompt}}` = full instruction text composed by
respex; `{{spec_path}}` = absolute path of the spec file.

- **refine** — instructions: read the spec at the given path, inspect the
  repository to ground the critique in what exists, fix ambiguity,
  contradictions, and gaps, improve structure without changing intent, rewrite
  the spec file in place, and touch no other files.
- **apply** — instructions: read the spec fully; make the repository conform;
  the spec is immutable; finish with a short summary of changes.
- **draft** (used by `new <description>`) — instructions: write a spec file at
  the given path for the described idea, following the skeleton sections.

`{{spec_path}}` keeps argv small: agents read the file themselves. Apply
passes the *committed* spec — guard 1 guarantees the working file is
byte-identical to it.

## 7. Agent Invocation Mechanics

1. Substitute placeholders in each argv element (`{{prompt}}`, `{{spec_path}}`).
2. `exec.Cmd`: argv, `Dir` = repo root, env = parent + config extras.
3. stdout and stderr each tee to the terminal and to a repo-relative log path
   under `.respex/logs/`: `<applies.id>-apply.log` for apply; UTC timestamp
   (`20060102T150405Z-<command>.log`) for refine/draft; stored in
   `applies.log_path`.
4. Success = exit code 0. stderr/stdout content is advisory only.
5. Ctrl+C / termination mid-run: the apply row keeps `NULL` `finished_at` /
   `exit_code`; the next `apply` warns "previous apply did not finish" and
   proceeds — every apply is an idempotent re-run.
6. Agent binary not found → named error showing the resolved command.

The agent decides its own edit mechanics (interactive permission prompts,
sandboxing, git behavior). `respex` inherits whatever the adapter's CLI does.

## 8. Error Handling

| Situation | Behavior |
|---|---|
| `.respex/` not found (upward walk exhausted) | Error: "not a respex project — run `respex new`" |
| Spec file missing/empty | Per-command error naming the configured path |
| `apply` with working spec ≠ last commit | Exit 1 with commit-first guidance |
| `apply` on already-applied version | One-line notice, exit 0, no agent run |
| `diff` before any commit | Error: "no committed versions yet" |
| `[agent] command` empty/unresolvable | Error naming the missing binary |
| Substituted argv > 32 KiB | Error suggesting `delivery = "stdin"` |
| SQLite open/migration failure | Error; never auto-delete the database |
| Interrupted apply (NULL exit) | Warning on next apply; safe to re-run |

Exit codes: 0 success or successful no-op; 1 any error. Nothing ever prompts
for input.

## 9. Code Layout and Dependencies

```
main.go                    — wiring only
internal/cli/              — stdlib flag.NewFlagSet per subcommand, dispatch table
internal/state/            — schema, migrations, queries (spec_versions, applies)
internal/spec/             — path resolution, hashing, skeleton template
internal/agent/            — config→adapter resolution, prompt building, exec+tee
internal/ui/               — output helpers: colored diff, status/log formatting
testdata/fakeagent/        — integration-test agent stub
```

Dependencies: `modernc.org/sqlite`, `github.com/pelletier/go-toml/v2`,
`github.com/pmezard/go-difflib` (unified diff). Go ≥ 1.24. No cgo, no cobra,
no shell-out plumbing. License: MIT.

## 10. Testing

- **Unit** — state package: CRUD, no-op predicate (last committed vs
  successful apply), migration; spec hashing; placeholder substitution and the
  32 KiB argv rule; config merge precedence.
- **Integration** — build `testdata/fakeagent` (a Go program that appends its
  prompt to a file and exits 0 or 1 per flag) into a temp dir, point
  `[agent] command` at it, and drive the full lifecycle end to end: new →
  refine → commit → apply (file touched) → apply again (no-op) → failing
  apply (retry allowed). Same suite runs on all three OSes in CI.
- **CI matrix** — linux/darwin/windows × amd64/arm64: build + unit + integration.
- **Manual smoke checklist** (README): one documented run per real agent CLI;
  kept out of CI because agent CLIs require interactive authentication.

## 11. Distribution

- GitHub repository `respex`; goreleaser on tag push produces static binaries
  (`CGO_ENABLED=0`) for all six targets (linux/darwin/windows × amd64/arm64)
  plus checksums.
- GitHub Actions: PR workflow (test + build matrix), release workflow (goreleaser).
- Homebrew tap added post-v1; install script may mirror goreleaser output.

## 12. Naming and Prior Art

- Name `respex`: "re-spec" iteration built in; CLI/npm/GitHub collision check
  found no dev-tool conflicts (RespexTech HR SaaS and a Japanese outsourcing
  firm are unrelated markets; the RESPEX trademark is dead, eyewear class).
- Prior art examined: `spex-lang/spex` (HTTP API spec language — different
  problem) and SpexCode (`spex` command; spec tree + worktrees + dashboard —
  heavier model this tool deliberately does not adopt).