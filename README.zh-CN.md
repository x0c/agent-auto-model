# cursor-mode-model

[![CI](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/cursor-mode-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

语言： [English](README.md) | 简体中文

按 Cursor Agent CLI 的 **Mode** 自动切换模型——Plan → Claude Opus，其它 → Grok——并且作用在真实请求路径上（不只是模型选择器 UI）。

**支持平台：** macOS · Linux · Windows

## 功能

- 按 Mode 映射模型 ID（`plan` / `default` / `search` / `debug`）
- 在真实发送路径强制映射模型（恢复会话也安全）
- 支持从 GitHub Releases 静默自更新，并可在 `status` 里诊断
- 配置命令：`show` / `set` / `set-many` / `enable` / `disable` / `set-strict` / `set-auto-update` / `set-update-interval` / `reset`（支持 `--json`）
- 可选 `strict`：校正失败则中止发送
- 显式 `--model` 会锁定本会话（不再自动切换）
- `status` 可查看近期决策审计

## 前置条件

需要已安装并登录 **Cursor Agent CLI**：

```bash
curl https://cursor.com/install -fsS | bash
```

本工具本身不计费；模型费用走你的 Cursor 账号。

## 安装

### macOS / Linux（Homebrew）

```bash
brew install x0c/tap/cursor-mode-model
cursor-mode-model install
```

### macOS / Linux（一键脚本）

```bash
curl -fsSL https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.sh | bash
```

### Windows（PowerShell）

```powershell
irm https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.ps1 | iex
```

### 备选（`go install`）

```bash
go install github.com/x0c/cursor-mode-model/cmd/cursor-mode-model@latest
cursor-mode-model install
```

安装后请**新开终端**，让 PATH 包装生效。

## 快速开始

```bash
cursor-mode-model status
cursor-mode-model config set plan claude-opus-5-thinking-high
cursor-mode-model config set default 'cursor-grok-*-high'
agent --mode plan
```

用 `status` 确认已生效（`active=true`），并开一个**新的** Agent 会话。安装/改配置之前就挂着的旧会话需要重启。

## 配置命令

```text
cursor-mode-model config show [--json]
cursor-mode-model config set <mode> <model-id> [--json]
cursor-mode-model config set-many plan=... default=... [--json]
cursor-mode-model config enable|disable [--json]
cursor-mode-model config set-strict true|false [--json]
cursor-mode-model config set-auto-update true|false [--json]
cursor-mode-model config set-update-interval <hours> [--json]
cursor-mode-model config reset [--json]
cursor-mode-model update [--force] [--quiet] [--json]
```

合法 mode：`plan`、`default`、`search`、`debug`。  
CLI 的 `--mode ask` 对应内部 `search`。

模型 ID 支持 shell 通配符（`*`、`?`）。发送前会对照 Cursor Agent 当前可用模型展开，并自动选**最新版本**（同版本优先非 `-fast`）。

默认映射：

| Mode | 模型 |
|---|---|
| Plan | `claude-opus-5-thinking-high` |
| Agent / Ask / Debug | `cursor-grok-*-high`（自动最新） |

## 静默自更新

- 默认每 24 小时按节流策略检查一次自更新。
- `agent` / `cursor-agent` 包装器会拉起独立后台进程检查更新，不阻塞正常请求启动。
- 更新源固定为当前平台对应的 GitHub Release 预编译包；下载成功后会自动再跑一次 `install`，刷新包装与资产。
- 更新失败时故障开放：当前命令继续执行，诊断信息写进 `status`。

常用命令：

```bash
cursor-mode-model status
cursor-mode-model update --force
cursor-mode-model config set-auto-update false
cursor-mode-model config set-update-interval 12
```

## 工作原理（简述）

1. 安装后在 PATH 最前放一层 `agent` / `cursor-agent` 包装。
2. 包装注入 Node 预加载，改写 Agent 打包后的 JS。
3. Mode 以会话权威元数据为准（含 resume）。
4. 发送前强制映射模型（除非被 `--model` 锁定）。

## 限制

- 不绑定 explore / 子代理模型（刻意为之）。
- 安装或改配置前已在跑的会话需要重启。
- 若 Cursor Agent 升级导致锚点失效，工具会故障开放（Agent 仍可运行，自动切换暂停）。用 `status` 诊断。
- 静默自更新只会更新 `cursor-mode-model` 自身，不会升级 Cursor Agent。

## 常见问题

**会改 Cursor 桌面端吗？**  
不会。只包装本机 PATH 上的 Agent CLI 入口。

**会覆盖 `--model` 吗？**  
不会。显式 `--model` 会锁定该会话。

**为什么装完 `status` 仍显示未生效？**  
多半是包装还没排到 PATH 最前——请新开终端，或确认包装目录已置顶。

**要花钱吗？**  
只有正常的 Cursor 模型用量。本项目 MIT，免费。

## 卸载

```bash
cursor-mode-model uninstall
```

## 许可证

[MIT](LICENSE)
