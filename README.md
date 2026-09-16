# respex

Spec-driven agentic development: one markdown spec per repo, applied by any agent CLI.

`respex` manages the lifecycle of a design spec and dispatches any agentic CLI
(Claude Code, Gemini CLI, Codex, aider, opencode, amp, …) to act on it:

    new → refine → diff → commit → apply

The spec is the durable source of truth; the codebase converges to it.

## Install

### GitHub Releases

Download the binary for your operating system and architecture from
[GitHub Releases](../../releases), then place it on your `PATH`.

Prebuilt binaries are available for Linux, macOS, and Windows on amd64 and
arm64.

### Build from source

Clone the repository and run:

```text
go build .
```

## Quick start

    respex new "a CLI that converts CSV to JSON"   # scaffold + agent-drafted spec
    respex refine                                   # agent improves the spec (repo-aware)
    respex diff --refine latest                     # review exactly what the agent changed
    respex diff                                     # compare working spec with last commit
    respex commit -m "initial spec"                 # snapshot as approved version
    respex apply                                    # agent makes the repo match the spec
    respex apply                                    # → "nothing to do (v1 already applied)"

Edit `SPEC.md` by hand any time; `commit` snapshots whatever is there.

`respex log` shows versions, refinements, and applies; `respex status`
summarizes the current spec and operation state. `respex restore` recovers a
saved refinement snapshot.

## Configuration

Create a user-level template and inspect its location with:

```text
respex config init
respex config path
```

The user config provides defaults for every project. `.respex/config.toml`
(project) overrides `~/.config/respex/config.toml` (user) key by key:

```toml
spec = "SPEC.md"
agent_timeout = "1h"                       # hard limit; Go duration syntax

[agent]
command = ["claude", "-p", "{{prompt}}"]   # argv array — any agentic CLI
delivery = "argv"                          # or "stdin" for long prompts
env = []
```

For example, a user-level Devin command can be replaced in one project by
putting only this in `.respex/config.toml`:

```toml
[agent]
command = ["claude", "-p", "{{prompt}}"]
```

Other user-level settings continue to be inherited. Placeholders: `{{prompt}}`
(instruction text) and `{{spec_path}}` (spec file path).
Agent CLIs must use their non-interactive mode; for Devin CLI, use
`command = ["devin", "--print", "{{prompt}}"]`. Success = exit code 0. Run
output is captured under `.respex/logs/`. The timeout
applies to both refine and apply. Only one apply, refine, or restore can run per
project; the lock is released automatically when Respex exits.

## State

`.respex/state.db` (SQLite, gitignored) stores spec snapshots, refinement history,
and apply history. Refinement is repeatable; `respex status` shows the count and
latest result, while `respex log` shows every run. Each refinement saves the spec
before and after the agent runs. Review or restore one with:

```text
respex diff --refine latest
respex diff --refine 3
respex restore --refine 3 --before
```

Restore operations also create snapshots, so they can be reversed. The database
is local-only and gitignored; refinement history does not follow the repository
to another machine. Commit important spec changes to Git.

On Linux and macOS, cancellation terminates the agent process group. On Windows,
only the direct agent process is terminated, so descendants may need manual cleanup.

## Manual smoke test (per agent CLI)

1. `respex new "demo"` in a scratch repo
2. Set `[agent] command` for your CLI
3. `respex refine` → `respex diff --refine latest` → review the proposal
4. `respex commit` → `respex apply` → changes appear; review with `git diff`
5. `respex apply` again → "nothing to do"

## License

MIT