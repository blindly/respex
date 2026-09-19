---
layout: default
title: Verification
nav_order: 9
---

# Verification

Verification closes the loop between the spec and the codebase. An `apply` run
succeeds when the agent exits 0 — only the code itself can say whether the
result still builds and passes its tests. ReSpex records conformance checks and
lets them drive `apply`.

## Configure

```toml
[verify]
commands = [["go", "build", "./..."], ["go", "test", "./..."]]
timeout = "30m"                # hard limit for the whole run; Go duration syntax
env = ["CGO_ENABLED=0"]        # extra environment for verify commands
audit = false                  # also let the configured agent audit conformance
```

With no commands and audit disabled, verification is off and behavior is
unchanged. `[verify]` keys merge across user and project config like
`[agent]`: an empty list or string means "not set" and cannot unset an
inherited value.

## Run

```text
respex verify
```

Verify refuses to run while the working spec differs from the last committed
version — commit first, so the result refers to an approved version. Commands
run in order with the project root as working directory and stop at the first
failure; the rest are recorded as skipped. Combined output is written to
`.respex/logs/<id>-verify.log`. The exit code is 0 only when every check
passed. Add `--json` for machine-readable results.

## Agent audit

Command checks capture what is mechanically testable. Some requirements are
behavioral — an agent can judge those. Set `audit = true` under `[verify]` and
ReSpex asks the configured agent to audit conformance after the command checks
pass:

```toml
[verify]
commands = [["go", "test", "./..."]]
audit = true
```

The audit agent receives a read-only prompt and an immutable snapshot of the
committed spec (the same mechanism `apply` uses), then inspects the repository.
It must end its reply with a final line `CONFORMS: yes` or `CONFORMS: no`;
ReSpex treats `no` as a failed check. A missing verdict, a non-zero agent exit
without a verdict, or a start failure is recorded as an `error` outcome. The
agent's full output is appended to the run log. Override the prompt with
`[prompts] verify = "..."` — keep the `CONFORMS` last line, it is the
contract.

The audit shares the run's single `[verify]` timeout budget: whatever time
remains after the command checks applies to the audit. When a command fails
first, the audit is skipped and recorded as such. `respex verify --json`
includes the audit result under an `audit` key.

The audit reuses the same `[agent]` command as apply, so the agent reviews its
own work. Treat it as advisory and keep everything mechanically checkable in
`commands` — those are the hard gate. Routing the audit to a different agent
than the apply agent is future work.

## How apply uses it

- After a whole-version `apply`, ReSpex chains the verification run. The apply
  row records the agent's success, but `respex apply` exits non-zero when the
  checks fail.
- A later `apply` inspects the latest verification for the committed version.
  When verification is configured and it did not pass (failed, errored, timed
  out, was interrupted, or is missing), apply re-runs the agent instead of
  reporting "nothing to do (vN already applied)":
  `v1 was applied but verification failed — re-applying`.
- Fix the working tree (or the checks) and run `respex verify` — or just
  `respex apply` again — to restore conformance.

`respex apply <feature>` never chains verification; feature-level checks are
out of scope. When verification is not configured, `apply` behaves exactly as
before.

## What is recorded

Each run adds a row to `.respex/state.db`, visible through:

```text
respex log                 # verifications section
respex status              # verified: line
respex verify --json
```

A row records the outcome (passed, failed, error, timed_out, interrupted,
stale), the per-command results, the audit result when audit is configured
(status, exit code, verdict, and the auditing agent named on the row), the log
path, and — when the project is a git repository — the HEAD commit and whether
the working tree was dirty. Git information is recorded for audit only; it
never gates a run.

## Notes

- Commands inherit the environment plus `[verify] env`; use it for flags like
  `GOFLAGS=-count=1` to defeat test caching.
- The whole run shares one timeout; a command that exceeds it is killed and
  the outcome is `timed_out`. The audit runs inside the same budget.
- The audit prompt is read-only: it instructs the agent not to modify files,
  and the agent reads the committed spec snapshot, not the working copy.
- `Ctrl-C` interrupts the current command; the outcome is `interrupted` and
  the exit code is 1.
- Only one apply, verify, refine, restore, or edit can run per project
  (`.respex/operation.lock`).