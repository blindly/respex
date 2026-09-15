# respex

Spec-driven agentic development: one markdown spec per repo, applied by any agent CLI.

`respex` manages the lifecycle of a design spec and dispatches any agentic CLI
(Claude Code, Gemini CLI, Codex, aider, opencode, amp, …) to act on it:

    new → refine → diff → commit → apply

The spec is the durable source of truth; the codebase converges to it.

## Install

Download a binary from [Releases](../../releases) (linux/darwin/windows, amd64/arm64),
or build from source: `go install github.com/YOU/respex@latest`
(`YOU` is a placeholder — swap in the actual GitHub owner once the repo is published).

## Quick start

    respex new "a CLI that converts CSV to JSON"   # scaffold + agent-drafted spec
    respex refine                                   # agent improves the spec (repo-aware)
    respex diff                                     # what changed since last commit
    respex commit -m "initial spec"                 # snapshot as approved version
    respex apply                                    # agent makes the repo match the spec
    respex apply                                    # → "nothing to do (v1 already applied)"

Edit `SPEC.md` by hand any time; `commit` snapshots whatever is there.

Two more commands inspect state: `respex log` shows versions and applies;
`respex status` summarizes the spec and apply state.

## Configuration

`.respex/config.toml` (project) overrides `~/.config/respex/config.toml` (user):

```toml
spec = "SPEC.md"

[agent]
command = ["claude", "-p", "{{prompt}}"]   # argv array — any agentic CLI
delivery = "argv"                          # or "stdin" for long prompts
env = []
```

Placeholders: `{{prompt}}` (instruction text), `{{spec_path}}` (spec file path).
Success = exit code 0. Run output is captured under `.respex/logs/`.

## State

`.respex/state.db` (SQLite, gitignored) stores spec snapshots and apply history.
It is local-only; the portable artifact is the spec file itself.

## Manual smoke test (per agent CLI)

1. `respex new "demo"` in a scratch repo
2. Set `[agent] command` for your CLI
3. `respex refine` → spec updated in place
4. `respex commit` → `respex apply` → changes appear; `git diff` review
5. `respex apply` again → "nothing to do"

## License

MIT