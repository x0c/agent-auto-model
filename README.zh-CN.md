# cursor-mode-model

**Cursor Agent 命令行**的独立补丁：会话 Mode 一变就自动换模型。

| Mode | 默认模型 |
|---|---|
| Plan | Claude Opus 5（高推理） |
| Agent / Ask / Debug | Grok 4.5 |

与 pickup **无关**，不依赖 pickup。

## 安装

```bash
go install ./cmd/cursor-mode-model
cursor-mode-model install
```

新开终端（或重新加载 shell 配置）后：

```bash
cursor-mode-model status
agent
```

## 配置

`~/.config/cursor-mode-model/config.json`（可用文本编辑器改映射）。

## 关闭

- 临时：`CURSOR_MODE_MODEL=0 agent …`
- 卸包装：`cursor-mode-model uninstall`

启动时若已写明要用哪个模型，本会话不再自动改。
