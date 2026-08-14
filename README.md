# agent-auto-model

[![CI](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Languages: English | [简体中文](README.zh-CN.md)

Auto-switch **agent CLI** models by Mode. Currently:

- **Cursor Agent CLI** — Plan → Claude Opus, everything else → Grok, with real request-path enforcement
- **Codex CLI** — Plan → `gpt-5.6-sol:high`, everything else → `gpt-5.6-terra:medium`, by rewriting app-server JSON-RPC

`cursor-mode-model` remains a compatibility alias.

**Supported platforms:** macOS · Linux · Windows

## Features

- Per-runtime Mode → model maps (`cursor`: plan / default / search / debug; `codex`: plan / default)
- Cursor: enforce the mapped model on the real send path (resume-safe)
- Codex: intercept `turn/start` and `thread/settings/update` (Shift+Tab Plan mode included)
- Silent self-update from GitHub Releases with status diagnostics
- CLI config: `show` / `set` / `set-many` / `enable` / `disable` / `set-strict` / `set-auto-update` / `set-update-interval` / `reset` (+ `--json`)
- Optional `strict` mode (Cursor): abort send if correction fails
- Explicit `--model` / `-m` locks the session (no auto-switch)
- Audit log of recent decisions via `status`

## Prerequisites

Install the CLIs you actually use:

```bash
# Cursor Agent CLI
curl https://cursor.com/install -fsS | bash

# Codex CLI
npm install -g @openai/codex
```

This tool does not bill models by itself.

## Install

### macOS / Linux (Homebrew)

```bash
brew install x0c/tap/agent-auto-model
agent-auto-model install
```

`brew install x0c/tap/cursor-mode-model` still works (alias formula).

### macOS / Linux (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.ps1 | iex
```

### Fallback (`go install`)

```bash
go install github.com/x0c/cursor-mode-model/cmd/agent-auto-model@latest
agent-auto-model install
```

Open a **new terminal** after install so PATH wrappers take effect.

## Quick Start

```bash
agent-auto-model status
agent-auto-model config set cursor.plan claude-opus-5-thinking-high
agent-auto-model config set cursor.default 'cursor-grok-*-high'
agent-auto-model config set codex.plan gpt-5.6-sol:high
agent-auto-model config set codex.default gpt-5.6-terra:medium
agent --mode plan
codex
```

Verify with `status` (look for `active=true`) and start a **new** session. Long-lived sessions started before install/config changes need a restart.

Bare `config set plan …` still means `cursor.plan` (deprecated prefix-less form).

## Config CLI

```text
agent-auto-model status [--runtime cursor|codex|all] [--json]
agent-auto-model install [--runtime cursor|codex] [--dry-run] [--json]
agent-auto-model uninstall [--runtime cursor|codex] [--dry-run] [--json]
agent-auto-model config show [--json]
agent-auto-model config set <mode|runtime.mode> <model-id> [--json]
agent-auto-model config set-many plan=... codex.plan=... [--json]
agent-auto-model config enable|disable [--runtime cursor|codex] [--json]
agent-auto-model config set-strict true|false [--json]
agent-auto-model config set-auto-update true|false [--json]
agent-auto-model config set-update-interval <hours> [--json]
agent-auto-model config reset [--json]
agent-auto-model update [--force] [--quiet] [--json]
```

Cursor modes: `plan`, `default`, `search`, `debug` (`--mode ask` → `search`).  
Codex modes: `plan`, `default`. Codex specs are `model[:effort]`, e.g. `gpt-5.6-sol:high`.

Model IDs may use shell-style wildcards (`*`, `?`). At request time the tool expands them against currently available models and picks the **latest** version.

Default mapping:

| Runtime | Mode | Model |
|---|---|---|
| Cursor | Plan | `claude-opus-5-thinking-high` |
| Cursor | Agent / Ask / Debug | `cursor-grok-*-high` (auto latest) |
| Codex | Plan | `gpt-5.6-sol:high` |
| Codex | Default | `gpt-5.6-terra:medium` |

## How it works

### Cursor

1. Install puts thin wrappers ahead of official `agent` / `cursor-agent` on PATH.
2. Wrappers inject a Node preload that patches Cursor Agent’s bundled JS.
3. Mode is read from the session’s authoritative metadata (including resume).
4. Before a request is built, the mapped model id is forced (unless locked by `--model`).

### Codex

1. Install puts a `codex` wrapper on PATH.
2. Interactive TUI is launched as `codex --remote unix://<session-sock>` against a local app-server proxy.
3. The proxy rewrites `thread/settings/update` and `turn/start` from `collaborationMode` (Shift+Tab Plan included).
4. Non-interactive subcommands (`exec`, `review`, `mcp`, …) pass through unchanged.
5. If the proxy cannot start, the official `codex` is launched (fail-open).

## Limitations

- Cursor: Explore / subagent models are **not** bound (by design).
- Codex: `--remote` / app-server transport is experimental upstream; upgrades may require a tool update. Fail-open if the protocol drifts.
- Sessions already running before install or config changes must be restarted.
- If Cursor Agent ships a bundle that no longer matches our anchors, Cursor auto-switch pauses (Agent still runs). Check `status`.
- Self-update only replaces this tool; it does not upgrade Cursor Agent or Codex.

## FAQ

**Does this change the Cursor desktop app?**  
No. It only wraps CLI entrypoints on your PATH.

**Will it override `--model`?**  
No. An explicit `--model` / `-m` locks that session.

**Why does `status` say inactive after install?**  
Usually the wrapper is not first on PATH yet — open a new terminal. On Ubuntu login shells, `.profile` may re-prepend `~/.local/bin` after `.bashrc`; re-run `agent-auto-model install` and verify with `bash -l -c 'agent-auto-model status'`.

**Codex TUI still shows the old model?**  
Trust `~/.local/share/agent-auto-model/assets/codex-decisions.log`. After a successful rewrite, `thread/settings/updated` should carry the mapped `model` / `effort`. Restart the TUI after config changes.

## Uninstall

```bash
agent-auto-model uninstall
```

## License

[MIT](LICENSE)
