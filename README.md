# respex

Spec-driven agentic development: one markdown spec per repo, applied by any agent CLI.

Documentation: [https://blindly.github.io/respex](https://blindly.github.io/respex)
For a step-by-step lifecycle guide, see [End-to-end process](https://blindly.github.io/respex/end-to-end/).

`respex` manages the lifecycle of a design spec and dispatches any agentic CLI
(Claude Code, Gemini CLI, Codex, aider, opencode, amp, …) to act on it:

    init → refine → diff → commit → apply

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

    respex init "a CLI that converts CSV to JSON"   # scaffold + agent-drafted spec
    respex view                                     # view the configured spec
    respex edit                                     # open SPEC.md in your editor
    respex refine                                   # agent improves the spec (repo-aware)
    respex diff --refine latest                     # review exactly what the agent changed
    respex diff                                     # compare working spec with last commit
    respex commit -m "initial spec"                 # snapshot as approved version
    respex apply                                    # agent makes the repo match the spec
    respex apply                                    # → "nothing to do (v1 already applied)"

For an existing repository, derive an initial spec from observed code, tests,
documentation, and configuration:

```text
respex init
respex baseline --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex commit -m "baseline existing implementation"
```

Use `respex baseline --merge` when the spec already contains meaningful content.
The agent preserves established intent and records conflicts as open questions.
Draft, baseline, and refine operate on temporary candidates; `SPEC.md` is replaced
only after the agent succeeds. Apply always receives an immutable copy of the
committed spec, so external edits cannot change what a running apply implements.

### Multi-file specs

Larger codebases can split the spec into a bundle: `SPEC.md` stays the master
document (intent, scope, invariants, and a feature index) while durable product
capabilities live under `specs/`:

```text
SPEC.md
specs/authentication.md
specs/billing.md
```

`respex baseline --split` asks the agent to propose this layout inside
`.respex/proposals/` without touching the live spec. Review the whole proposal,
then accept or discard it:

```text
respex baseline --split --intent "what this project is meant to accomplish"
respex diff --baseline latest        # per-file diff of the proposal
respex baseline accept               # install files + configure spec_files
respex baseline discard              # drop the proposal
```

Accept refuses to run if any target file changed since the proposal was
generated, then writes `spec_files` into `.respex/config.toml`:

```toml
spec = "SPEC.md"
spec_files = ["specs/authentication.md", "specs/billing.md"]
```

Once configured, `commit` snapshots the whole bundle atomically, `diff`
compares it file by file, `status` and `doctor` track every file, and `apply`
hands the agent an immutable copy of the entire bundle. `view`, `edit`,
`refine`, and `restore` operate on the master spec.

Edit `SPEC.md` by hand any time; `commit` snapshots whatever is there.

`respex log` shows versions, baselines, refinements, and applies; `respex status`
summarizes the current spec and operation state. `respex restore` recovers a
saved refinement snapshot.

## Configuration

Create a user-level template and inspect its location with:

```text
respex config init                 # create user config
respex config edit                 # edit user config
respex config path                 # print user config path
respex config edit --local         # edit this project's overrides
respex config validate --local     # validate the merged project config
respex config show                 # show effective values and their sources
```

The user config provides defaults for every project. `.respex/config.toml`
(project) overrides `~/.config/respex/config.toml` (user) key by key:

```toml
spec = "SPEC.md"
spec_files = []                            # extra bundle files, e.g. specs/billing.md
editor = ["code", "--wait"]                # optional editor argv
pager = ["less", "-FRX"]                   # optional pager argv
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

`respex edit` uses the configured editor array, then `VISUAL`, then `EDITOR`,
and finally Notepad on Windows. GUI editors should include their wait argument.
`respex view` uses the configured pager, then `PAGER`, then `less`/`more`; pipes
and redirects always receive exact Markdown without decoration. Historical views:

```text
respex view --version 3
respex view --refine latest --before
respex view --baseline 2 --after
respex view --raw
```

Other user-level settings continue to be inherited. Placeholders: `{{prompt}}`
(instruction text) and `{{spec_path}}` (spec file path).
Agent CLIs must use their non-interactive mode; for Devin CLI, use
`command = ["devin", "--print", "{{prompt}}"]`. Success = exit code 0. Run
output is captured under `.respex/logs/`. Interactive terminals show an elapsed
time spinner during agent operations; use `--no-progress` or set `NO_COLOR` to
disable it. The timeout applies to drafting, refine, and apply. Only one apply,
refine, restore, or edit can run per project; the lock is released automatically
when Respex exits.

## State

`.respex/state.db` (SQLite, gitignored) stores spec snapshots, refinement history,
and apply history. Refinement is repeatable; `respex status` shows the count and
latest result, while `respex log` shows every run. Each refinement saves the spec
before and after the agent runs. Refining the
untouched generated skeleton is rejected, as is repeating the same spec, prompt,
and agent configuration after an unchanged result. Edit the spec first or use
`respex refine --force` to retry explicitly. Review or restore one with:

```text
respex diff --refine latest
respex diff --refine 3
respex restore --refine 3 --before
respex restore --baseline 2 --before
```

Restore operations also create snapshots, so they can be reversed. The database
is local-only and gitignored; refinement history does not follow the repository
to another machine. Commit important spec changes to Git.

On Linux and macOS, cancellation first interrupts the agent process group and
force-kills it after a grace period. On Windows, Respex uses `taskkill /T` to
terminate the agent process tree.

## Maintenance

```text
respex update --check              # check the latest GitHub release
respex update                      # verify checksum and replace this binary
respex update --version v0.1.1     # install a specific release
respex doctor                      # diagnose config, tools, state, and locks
respex spec validate               # check required spec sections
respex completion powershell       # generate shell completion
respex status --json               # machine-readable automation output
```

`status`, `log`, `config show`, `doctor`, and `update --check` accept `--json`.

Updates are installed only after the downloaded binary matches the release's
`checksums.txt`. A failed download or checksum leaves the executable unchanged.

## Manual smoke test (per agent CLI)

1. `respex init "demo"` in a scratch repo
2. Set `[agent] command` for your CLI
3. `respex refine` → `respex diff --refine latest` → review the proposal
4. `respex commit` → `respex apply` → changes appear; review with `git diff`
5. `respex apply` again → "nothing to do"

## License

MIT