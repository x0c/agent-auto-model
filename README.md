# cursor-mode-model

Independent shim for **Cursor Agent CLI**: when session Mode changes, automatically switch model — and **force the model on the real request path**, not only in the UI picker.

- Plan → `claude-opus-5-thinking-high`
- Agent / Ask / Debug → `cursor-grok-4.5-high-fast`

**Not part of pickup.** No dependency on pickup.

## Install

```bash
go install forgejo.caozc.top/Max/cursor-mode-model/cmd/cursor-mode-model@latest
# or from this repo:
go install ./cmd/cursor-mode-model
cursor-mode-model install
```

Open a new terminal (or `source` your shell rc), then:

```bash
cursor-mode-model status
agent   # wrapped; Mode changes switch models
```

## Config

`~/.config/cursor-mode-model/config.json`

```json
{
  "version": 1,
  "enabled": true,
  "models": {
    "plan": "claude-opus-5-thinking-high",
    "default": "cursor-grok-4.5-high-fast",
    "search": "cursor-grok-4.5-high-fast",
    "debug": "cursor-grok-4.5-high-fast"
  }
}
```

## Disable

```bash
CURSOR_MODE_MODEL=0 agent …
# or
cursor-mode-model uninstall
```

Explicit `--model …` locks auto-switch for that session.

## License

MIT
