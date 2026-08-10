# cursor-mode-model

[![CI](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Languages: English | [简体中文](README.zh-CN.md)

Auto-switch **Cursor Agent CLI** models by Mode — Plan → Claude Opus, everything else → Grok — with real request-path enforcement (not just the model picker UI).

**Supported platforms:** macOS · Linux · Windows

## Features

- Map Modes to model IDs (`plan` / `default` / `search` / `debug`)
- Enforce the mapped model on the real send path (resume-safe)
- CLI config: `show` / `set` / `set-many` / `enable` / `disable` / `set-strict` / `reset` (+ `--json`)
- Optional `strict` mode: abort send if correction fails
- Explicit `--model` locks the session (no auto-switch)
- Audit log of recent decisions via `status`

## Prerequisites

You must already have **Cursor Agent CLI** installed and logged in:

```bash
curl https://cursor.com/install -fsS | bash
```

This tool does not bill models by itself; usage is charged to your Cursor account.

## Install

### macOS / Linux (Homebrew)

```bash
brew install x0c/tap/cursor-mode-model
cursor-mode-model install
```

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
go install github.com/x0c/cursor-mode-model/cmd/cursor-mode-model@latest
cursor-mode-model install
```

Open a **new terminal** after install so PATH wrappers take effect.

## Quick Start

```bash
cursor-mode-model status
cursor-mode-model config set plan claude-opus-5-thinking-high
cursor-mode-model config set default cursor-grok-4.5-high-fast
agent --mode plan
```

Verify with `status` (look for `active=true`) and start a **new** Agent session. Long-lived sessions started before install/config changes need a restart.

## Config CLI

```text
cursor-mode-model config show [--json]
cursor-mode-model config set <mode> <model-id> [--json]
cursor-mode-model config set-many plan=... default=... [--json]
cursor-mode-model config enable|disable [--json]
cursor-mode-model config set-strict true|false [--json]
cursor-mode-model config reset [--json]
```

Valid modes: `plan`, `default`, `search`, `debug`.  
CLI `--mode ask` maps to internal `search`.

Default mapping:

| Mode | Model |
|---|---|
| Plan | `claude-opus-5-thinking-high` |
| Agent / Ask / Debug | `cursor-grok-4.5-high-fast` |

## How it works

1. Install puts thin wrappers ahead of the official `agent` / `cursor-agent` on your PATH.
2. Wrappers inject a Node preload that patches Cursor Agent’s bundled JS.
3. Mode is read from the session’s authoritative metadata (including resume).
4. Before a request is built, the mapped model id is forced (unless locked by `--model`).

## Limitations

- Explore / subagent models are **not** bound (by design).
- Sessions already running before install or config changes must be restarted.
- If Cursor Agent ships a bundle that no longer matches our anchors, the tool fails open (Agent still runs; auto-switch pauses). Check `status`.

## FAQ

**Does this change the Cursor desktop app?**  
No. It only wraps the Agent CLI entrypoints on your PATH.

**Will it override `--model`?**  
No. An explicit `--model` locks that session.

**Why does `status` say inactive after install?**  
Usually the wrapper is not first on PATH yet — open a new terminal, or ensure the wrapper bin directory is prepended.

**Any cost?**  
Only normal Cursor model usage. This project is free (MIT).

## Uninstall

```bash
cursor-mode-model uninstall
```

## License

[MIT](LICENSE)
