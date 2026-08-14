# agent-auto-model

[![CI](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

语言：[English](README.md) | 简体中文

按 **Mode** 自动切换各 Agent CLI 的模型。当前支持：

- **Cursor Agent CLI**：Plan → Claude Opus，其它 → Grok，作用在真实请求路径
- **Codex CLI**：Plan → `gpt-5.6-sol:high`，其它 → `gpt-5.6-terra:medium`，改写 app-server JSON-RPC

`cursor-mode-model` 仍是兼容别名。

**支持平台：** macOS · Linux · Windows

## 功能

- 分 runtime 的 Mode→模型映射（Cursor：`plan` / `default` / `search` / `debug`；Codex：`plan` / `default`）
- Cursor：在真实发送路径强制映射（resume 安全）
- Codex：拦截 `turn/start` 与 `thread/settings/update`（含 Shift+Tab 切 Plan）
- GitHub Releases 静默自更新，`status` 可诊断
- 配置命令支持 `--json`；显式 `--model` / `-m` 锁定本会话

## 前置条件

```bash
curl https://cursor.com/install -fsS | bash   # Cursor Agent CLI
npm install -g @openai/codex                 # Codex CLI
```

## 安装

```bash
brew install x0c/tap/agent-auto-model
agent-auto-model install
```

或：

```bash
curl -fsSL https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.sh | bash
```

Windows：

```powershell
irm https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.ps1 | iex
```

`go install github.com/x0c/cursor-mode-model/cmd/agent-auto-model@latest` 后同样要跑 `install`。装完请**新开终端**。

## 快速开始

```bash
agent-auto-model status
agent-auto-model config set cursor.plan claude-opus-5-thinking-high
agent-auto-model config set cursor.default 'cursor-grok-*-high'
agent-auto-model config set codex.plan gpt-5.6-sol:high
agent-auto-model config set codex.default gpt-5.6-terra:medium
```

未写 runtime 前缀的 `config set plan …` 仍视为 `cursor.plan`。Codex 规格为 `model[:effort]`。

```text
agent-auto-model status [--runtime cursor|codex|all] [--json]
agent-auto-model install [--runtime cursor|codex]
agent-auto-model config set <mode|runtime.mode> <model-id>
agent-auto-model config enable|disable [--runtime cursor|codex]
```

## 工作原理

- Cursor：PATH 包装注入 Node 预加载，打补丁后在 `buildRequestedModel` 强制模型。
- Codex：包装器把交互式 TUI 接到本机 app-server 代理（`codex --remote unix://…`），按 `collaborationMode` 改写 `model` / `effort`。`exec` / `review` 等非交互子命令原样透传。代理失败则故障开放，直接启动官方 `codex`。

## 卸载

```bash
agent-auto-model uninstall
```

## License

[MIT](LICENSE)
