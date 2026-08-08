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

Long-running sessions started before an upgrade must be **restarted** to pick up the new preload.

## Config

`~/.config/cursor-mode-model/config.json`

```json
{
  "version": 1,
  "enabled": true,
  "strict": false,
  "models": {
    "plan": "claude-opus-5-thinking-high",
    "default": "cursor-grok-4.5-high-fast",
    "search": "cursor-grok-4.5-high-fast",
    "debug": "cursor-grok-4.5-high-fast"
  }
}
```

- `strict: true` — if a send cannot be corrected to the Mode-mapped model, the turn is aborted with an error (default is warn-only).
- Decision audit log (metadata only): `~/.local/share/cursor-mode-model/assets/decisions.log`

## Guarantee (what is / isn’t covered)

| Covered | Not covered |
|---|---|
| Main chat turns: Mode → request model | Explore / subagent model (`exploreSubagentModel`) — left to Cursor |
| Resume without `--mode` (reads session Mode) | Sessions still running old preload (restart required) |
| UI + `lastUsedModel` kept in sync after force | |

## Disable

```bash
CURSOR_MODE_MODEL=0 agent …
# or
cursor-mode-model uninstall
```

Explicit `--model …` locks auto-switch for that session.

## License

MIT
