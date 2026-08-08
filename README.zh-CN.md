# cursor-mode-model

**Cursor Agent 命令行**的独立补丁：会话 Mode 一变就自动换模型，并在**真正发请求前**再强制一次（不只改界面选择器）。

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

升级后，**升级前就一直开着的会话需要重启**才会吃到新挂钩。

## 配置

`~/.config/cursor-mode-model/config.json`

- `strict: true`：发送前纠正失败时直接中断本轮（默认只告警不打断）
- 决策审计日志（不含对话正文）：`~/.local/share/cursor-mode-model/assets/decisions.log`

## 保证范围

会管：主对话按 Mode 发请求、接着聊旧会话时也能认出 Mode、界面与「上次模型」记忆跟真实值对齐。

不管：探索/子代理用哪颗模型（保持 Cursor 原样）；仍在跑旧挂钩的长期会话（需重启）。

## 关闭

- 临时：`CURSOR_MODE_MODEL=0 agent …`
- 卸包装：`cursor-mode-model uninstall`

启动时若已写明要用哪个模型，本会话不再自动改。
