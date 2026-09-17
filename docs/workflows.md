# Workflows

## New project

```text
respex init "a CLI that converts CSV to JSON"
respex refine
respex diff --refine latest
respex commit -m "initial spec"
respex apply
respex apply          # already applied
```

## Existing project

### Single-file baseline

```text
cd existing-project
respex init
respex baseline --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex commit -m "baseline existing implementation"
respex apply
```

Use `respex baseline --merge` if `SPEC.md` already has meaningful content.

### Multi-file baseline

For larger codebases, ask the agent to propose a spec bundle:

```text
respex init
respex baseline --split --intent "what this project is meant to accomplish"
respex diff --baseline latest
respex baseline accept
respex commit -m "multi-file baseline"
```

See [Multi-file specs](multi-file-specs.md) for details.

## Refine loop

```text
respex refine
respex diff --refine latest
respex commit -m "clarify auth requirements"
respex apply
```

Refinement is repeatable. Respex rejects a refine that would be a no-op:

- The spec is still the generated skeleton.
- The exact same spec, prompt, and agent configuration was last refined
  unchanged.

Use `respex refine --force` to override either guard.

## Review before applying

```text
respex diff          # working vs last commit
respex commit        # snapshot if acceptable
respex apply
```

`apply` refuses to run if the working spec differs from the last committed
version; commit first.

## Rollback

```text
respex diff --refine latest
respex restore --refine latest --before
respex diff --refine latest
```

Restore creates a reversible history entry, so you can change your mind.

## Cancel an agent run

On Linux and macOS, `Ctrl-C` first interrupts the agent process group and
force-kills it after a grace period. On Windows, Respex uses `taskkill /T` to
terminate the agent process tree. The live spec is not modified on failure,
timeout, or interruption.
