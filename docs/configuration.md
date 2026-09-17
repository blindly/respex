# Configuration

Respex merges configuration in this order:

```text
built-in defaults
       ↓
user config
       ↓
project .respex/config.toml
```

Project-level values override user-level values field by field. Arrays replace
wholesale; an empty array or string means "not set" and does not unset an
inherited value.

## Setup commands

```text
respex config init                 # create user config
respex config init --local         # create project config
respex config path                 # print user config path
respex config path --local         # print project config path
respex config edit                 # edit user config
respex config edit --local         # edit project config
respex config validate             # validate user config
respex config validate --local     # validate merged project config
respex config show                 # show effective values and sources
respex config show --json
```

## Example user config

```toml
spec = "SPEC.md"
spec_files = []                            # extra bundle files for large projects
editor = ["code", "--wait"]                # optional editor argv
pager = ["less", "-FRX"]                   # optional pager argv
agent_timeout = "1h"                         # hard limit; Go duration syntax

[agent]
command = ["claude", "-p", "{{prompt}}"]   # argv array — any agentic CLI
# command = ["devin", "--print", "{{prompt}}"]
# command = ["codex", "exec", "{{prompt}}"]
# command = ["gemini", "-p", "{{prompt}}"]
# command = ["opencode", "run", "{{prompt}}"]
delivery = "argv"                            # or "stdin" for long prompts
env = []

[prompts]
# Optional overrides for built-in prompts. {{prompt}} carries the description for
# draft/baseline; {{spec_path}} is replaced with the spec file or directory.
# draft    = "..."
# baseline = "..."
# refine   = "..."
# apply    = "..."
```

## Editor and pager

`respex edit` uses the configured editor, then `VISUAL`, then `EDITOR`. GUI
editors should include their wait argument (for example, `code --wait`).
`VISUAL`/`EDITOR` values with arguments are parsed safely.

`respex view` uses the configured pager, then `PAGER`, then `less -FRX` /
`more`. Piped or redirected output receives exact Markdown without decoration.

## Agent non-interactive mode

Agent CLIs must run non-interactively. For Devin CLI, use:

```toml
[agent]
command = ["devin", "--print", "{{prompt}}"]
```

Success means exit code 0. Output is captured under `.respex/logs/`. Interactive
terminals show an elapsed-time spinner; disable with `--no-progress` or `NO_COLOR`.
