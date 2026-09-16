# respex

Spec-driven agentic development: one markdown spec per repo, applied by any agent CLI.

`respex` manages the lifecycle of a design spec and dispatches any agentic CLI
(Claude Code, Gemini CLI, Codex, aider, opencode, amp, …) to act on it:

    new → refine → diff → commit → apply

The spec is the durable source of truth; the codebase converges to it.

## Install

### macOS / Linux

```bash
TAG=$(curl -sSL https://api.github.com/repos/blindly/respex/releases/latest | grep '"tag_name":' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac
case "$OS" in linux) OS=linux ;; darwin) OS=darwin ;; cygwin*|msys*|mingw*) OS=windows ;; esac
URL="https://github.com/blindly/respex/releases/download/${TAG}/respex_${TAG}_${OS}_${ARCH}"
tmp=$(mktemp)
if curl -fsSL -o "$tmp" "${URL}.exe" 2>/dev/null; then
  install "$tmp" "${HOME}/.local/bin/respex.exe"
else
  curl -fsSL -o "$tmp" "$URL"
  install "$tmp" "${HOME}/.local/bin/respex"
fi
```

Change the install destination (`${HOME}/.local/bin`) to `/usr/local/bin` or any
other directory on your `PATH`.

### Windows (PowerShell)

```powershell
$ErrorActionPreference = "Stop"
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/blindly/respex/releases/latest"
$tag = $release.tag_name
$arch = switch ($env:PROCESSOR_ARCHITECTURE) { "AMD64" { "amd64" } "ARM64" { "arm64" } }
$base = "https://github.com/blindly/respex/releases/download/${tag}/respex_${tag}_windows_${arch}"
$tmp = New-TemporaryFile
try {
    Invoke-RestMethod -Uri "${base}.exe" -OutFile "$($tmp.FullName).exe"
    Copy-Item "$($tmp.FullName).exe" "$env:LOCALAPPDATA\Microsoft\WindowsApps\respex.exe" -Force
} catch {
    Invoke-RestMethod -Uri "$base" -OutFile $tmp.FullName
    Copy-Item $tmp.FullName "$env:LOCALAPPDATA\Microsoft\WindowsApps\respex.exe" -Force
}
```

### Go install

If you have Go installed:

```bash
go install github.com/blindly/respex@latest
```

### Manual

Download a binary from [Releases](../../releases) (linux/darwin/windows,
amd64/arm64).

### Build from source

```bash
git clone https://github.com/blindly/respex.git
cd respex
go build .
```

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