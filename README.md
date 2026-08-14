# agent-auto-model

[![CI](https://github.com/x0c/agent-auto-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/agent-auto-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Languages: English | [简体中文](README.zh-CN.md)

When you switch **Mode** in Cursor Agent CLI or Codex CLI, this tool switches the **model** for you. After install, keep using `agent` / `codex` as usual.

**Supported platforms:** macOS · Linux · Windows

This tool does not bill models. You still need Cursor Agent CLI and/or Codex CLI, plus whatever those tools charge.

## Out of the box

| Tool | Plan | Everything else |
|---|---|---|
| Cursor Agent CLI | `claude-opus-*-thinking-high` (latest matching Opus High) | `cursor-grok-*-high` (latest matching Grok High) |
| Codex CLI | `gpt-*-sol:high` (latest matching Sol High) | `gpt-*-terra:medium` (latest matching Terra Medium) |

Cursor also maps Ask → `search` and Debug → `debug` (same default as Agent). Codex only has Plan vs Default.

These mappings follow the repo’s recommended table by default and update in the background. Changing a mapping switches you to a local copy that no longer follows the repo. Already-open sessions need a restart to pick up a new mapping.

## Install

First install the CLIs you actually use:

```bash
curl https://cursor.com/install -fsS | bash   # Cursor Agent CLI
npm install -g @openai/codex                 # Codex CLI
```

### macOS / Linux (Homebrew)

```bash
brew install x0c/tap/agent-auto-model
agent-auto-model install
```

### macOS / Linux (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/x0c/agent-auto-model/main/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/x0c/agent-auto-model/main/install.ps1 | iex
```

### Fallback (`go install`)

```bash
go install github.com/x0c/agent-auto-model/cmd/agent-auto-model@latest
agent-auto-model install
```

Open a **new terminal**, then check:

```bash
agent-auto-model status
```

You want `active=true`. Then start a **new** `agent` or `codex` session. Sessions already running will not pick up the install.

## Usage

### 1. Use it (nothing extra)

```bash
agent --mode plan
agent
codex
```

Switch Mode as you normally would (Cursor Mode, Codex Shift+Tab Plan). The mapped model is applied on the real request path.

### 2. See the current mapping

```bash
agent-auto-model config show
agent-auto-model status
```

### 3. Change a default model

Replace the model id with whatever you want. These write to your config and stay until you change them again.

```bash
# Cursor: Plan / Agent / Ask / Debug
agent-auto-model config set cursor.plan 'claude-opus-*-thinking-high'
agent-auto-model config set cursor.default 'cursor-grok-*-high'
agent-auto-model config set cursor.search 'cursor-grok-*-high'
agent-auto-model config set cursor.debug 'cursor-grok-*-high'

# Codex: Plan / Default  (value is model[:effort])
agent-auto-model config set codex.plan 'gpt-*-sol:high'
agent-auto-model config set codex.default 'gpt-*-terra:medium'

# Several at once
agent-auto-model config set-many cursor.plan='claude-opus-*-thinking-high' codex.default='gpt-*-terra:medium'
```

`config set plan …` (no prefix) still means `cursor.plan`. Codex **must** use `codex.plan` / `codex.default`.

Model ids may include `*` / `?`. At request time the tool expands them against currently available models and picks the latest version.

Changing a mapping switches you to a local copy that no longer follows the repo defaults. To follow them again:

```bash
agent-auto-model config set-models-source recommended
```

Restart any session that was already open.

### 4. Turn auto-switch off (keep the tool installed)

```bash
# Both Cursor and Codex
agent-auto-model config disable

# One side only
agent-auto-model config disable --runtime cursor
agent-auto-model config disable --runtime codex

# Turn it back on
agent-auto-model config enable
```

This does **not** uninstall. Your mappings stay. Official CLIs still run; they just stop being rewritten.

For the current shell only: `AGENT_AUTO_MODEL=0`.

### 5. Pin a model for this session only

Does **not** change saved defaults.

```bash
agent --model claude-opus-5-thinking-high
codex --model gpt-5.6-sol:high
codex -m gpt-5.6-sol:high          # Codex short flag; Cursor wrapper does not honor -m
```

In Codex TUI, picking a different model with `/model` also locks that session.

### 6. Uninstall (remove the wrappers)

```bash
agent-auto-model uninstall
```

Use this when you want `agent` / `codex` to be the official binaries again. To only pause switching, use `config disable`.

Restore factory mappings:

```bash
agent-auto-model config reset
```

## Command reference

| Command | What it does |
|---|---|
| `status` | Is wrapping actually on PATH? Is switching on? |
| `config show` | Print saved Mode → model maps |
| `config set <runtime.mode> <model>` | Change one mapping (also switches to a local copy) |
| `config set-many a=… b=…` | Change several mappings (also switches to a local copy) |
| `config set-models-source recommended\|local` | Follow the repo table, or keep a local copy |
| `config refresh-recommended` | Fetch the latest recommended table now |
| `config disable` / `enable` | Pause / resume auto-switch (`--runtime cursor\|codex` for one side) |
| `config set-strict true\|false` | Cursor: abort the send if correction fails |
| `config set-auto-update true\|false` | Silent self-update from GitHub Releases |
| `config set-update-interval <hours>` | How often to check for updates |
| `config reset` | Factory maps and switches |
| `install` / `uninstall` | Install or remove PATH wrappers |
| `update` | Update this tool now |

Add `--json` (or run without a TTY) for machine-readable output.

## How it works

**Cursor:** install puts wrappers ahead of official `agent` / `cursor-agent` on PATH. A Node preload forces the mapped model before the request is built (resume-safe). Explore / subagent models are left alone.

**Codex:** the `codex` wrapper proxies the interactive TUI and rewrites `model` / `effort` from Plan vs Default (including Shift+Tab). `exec` / `review` / `mcp` pass through. If the proxy cannot start, official `codex` runs unchanged.

## Limitations

- Sessions started before install or config changes must be restarted.
- If Cursor Agent’s bundle no longer matches our patch points, Cursor auto-switch pauses (Agent still runs). Check `status`.
- Codex `--remote` is experimental upstream; protocol drift fail-opens.
- Self-update only replaces this tool, not Cursor Agent or Codex.
- `cursor agent` bypasses the wrapper. Use `agent` or `cursor-agent`.

## FAQ

**Does this change the Cursor desktop app?**  
No. It only wraps CLI entrypoints on your PATH.

**Will it override `--model`?**  
No. That flag locks the session and does not change saved defaults.

**`status` says inactive after install?**  
Open a new terminal so PATH picks up the wrapper. On Ubuntu login shells, `.profile` may re-prepend `~/.local/bin` after `.bashrc` — re-run `agent-auto-model install`, then `bash -l -c 'agent-auto-model status'`.

**Codex TUI still shows the old model?**  
Check `~/.local/share/agent-auto-model/assets/codex-decisions.log`. After a rewrite you should see the mapped `model` / `effort`. Restart the TUI after config changes.

## License

[MIT](LICENSE)
