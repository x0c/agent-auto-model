# agent-auto-model

[![CI](https://github.com/x0c/agent-auto-model/actions/workflows/test.yml/badge.svg)](https://github.com/x0c/agent-auto-model/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

语言：[English](README.md) | 简体中文

在 Cursor Agent CLI 或 Codex CLI 里切换 **Mode** 时，自动换对应的**模型**。装好之后照常敲 `agent` / `codex` 即可。

**支持平台：** macOS · Linux · Windows

本工具本身不计费。你仍需要先装 Cursor Agent CLI 和/或 Codex CLI，费用按它们自己的规则走。

## 装好后默认怎么切

| 工具 | Plan | 其它 Mode |
|---|---|---|
| Cursor Agent CLI | `claude-opus-5-thinking-high` | `cursor-grok-*-high`（自动选当前最新的 Grok High） |
| Codex CLI | `gpt-5.6-sol:high` | `gpt-5.6-terra:medium` |

Cursor 还会把 Ask 记成 `search`、Debug 记成 `debug`（默认模型和 Agent 一样）。Codex 只有 Plan 和 Default。

默认跟随仓库里的推荐表，后台会自动更新。自己改过映射后会切成「本地自定义」，不再跟随仓库。已经开着的对话要重开一次才吃到新映射。

## 安装

先装你实际会用的 Agent CLI：

```bash
curl https://cursor.com/install -fsS | bash   # Cursor Agent CLI
npm install -g @openai/codex                 # Codex CLI
```

### macOS / Linux（Homebrew）

```bash
brew install x0c/tap/agent-auto-model
agent-auto-model install
```

### macOS / Linux（一条命令）

```bash
curl -fsSL https://raw.githubusercontent.com/x0c/agent-auto-model/main/install.sh | bash
```

### Windows（PowerShell）

```powershell
irm https://raw.githubusercontent.com/x0c/agent-auto-model/main/install.ps1 | iex
```

### 备用（`go install`）

```bash
go install github.com/x0c/agent-auto-model/cmd/agent-auto-model@latest
agent-auto-model install
```

装完请**新开终端**，然后检查：

```bash
agent-auto-model status
```

看到 `active=true` 再新开一次 `agent` 或 `codex`。已经开着的对话不会吃到这次安装。

## 用法

### 1. 日常使用（不用再敲别的）

```bash
agent --mode plan
agent
codex
```

Mode 还是你平时那样切（Cursor 的 Mode、Codex 的 Shift+Tab Plan）。对应模型会在真正发请求时换上。

### 2. 看当前映射

```bash
agent-auto-model config show
agent-auto-model status
```

### 3. 改某个 Mode 的默认模型

把模型 ID 换成你想要的。写入配置后一直生效，直到你再改。

```bash
# Cursor：Plan / Agent / Ask / Debug
agent-auto-model config set cursor.plan claude-opus-5-thinking-high
agent-auto-model config set cursor.default 'cursor-grok-*-high'
agent-auto-model config set cursor.search 'cursor-grok-*-high'
agent-auto-model config set cursor.debug 'cursor-grok-*-high'

# Codex：Plan / Default（值为 模型[:力度]）
agent-auto-model config set codex.plan gpt-5.6-sol:high
agent-auto-model config set codex.default gpt-5.6-terra:medium

# 一次改多条
agent-auto-model config set-many cursor.plan=claude-opus-5-thinking-high codex.default=gpt-5.6-terra:medium
```

不写前缀的 `config set plan …` 仍当成 `cursor.plan`。Codex **必须**写成 `codex.plan` / `codex.default`。

模型 ID 可以用 `*` / `?`。发请求时会按当前可用模型展开，并选最新版本。

改过映射后，来源会变成「本地自定义」，不再跟随仓库推荐配置。切回去：

```bash
agent-auto-model config set-models-source recommended
```

已经开着的对话要重开一次才生效。

### 4. 关掉自动切换（工具还留着）

```bash
# Cursor 和 Codex 一起关
agent-auto-model config disable

# 只关一边
agent-auto-model config disable --runtime cursor
agent-auto-model config disable --runtime codex

# 再打开
agent-auto-model config enable
```

这不是卸载。映射还在。官方 CLI 照常能跑，只是不再被改模型。

只对当前终端生效：`AGENT_AUTO_MODEL=0`。

### 5. 只锁这一次对话（不改默认）

```bash
agent --model claude-opus-5-thinking-high
codex --model gpt-5.6-sol:high
codex -m gpt-5.6-sol:high          # Codex 短参数；Cursor 包装不认 -m
```

Codex 界面里用 `/model` 选了和映射不一样的模型，这次对话同样会锁住。

### 6. 卸载（拿掉包装）

```bash
agent-auto-model uninstall
```

想让 `agent` / `codex` 完全回到官方入口时用这个。只是暂时不想自动切，用 `config disable`。

恢复出厂映射：

```bash
agent-auto-model config reset
```

## 命令一览

| 命令 | 做什么 |
|---|---|
| `status` | 包装有没有排到 PATH 前面、自动切换开没开 |
| `config show` | 打印已保存的 Mode → 模型 |
| `config set <runtime.mode> <模型>` | 改一条映射（同时切成本地自定义） |
| `config set-many a=… b=…` | 一次改多条（同时切成本地自定义） |
| `config set-models-source recommended\|local` | 跟随仓库推荐，或改用本地自定义 |
| `config refresh-recommended` | 立刻拉取最新推荐表 |
| `config disable` / `enable` | 关掉 / 打开自动切换（可加 `--runtime cursor\|codex`） |
| `config set-strict true\|false` | Cursor：纠正失败就阻断这次发送 |
| `config set-auto-update true\|false` | 是否从 GitHub Releases 静默自更新 |
| `config set-update-interval <小时>` | 检查更新的间隔 |
| `config reset` | 恢复出厂映射和开关 |
| `install` / `uninstall` | 安装或移除 PATH 包装 |
| `update` | 立刻更新本工具 |

加 `--json`（或没有终端时）会输出 JSON。

## 工作原理

**Cursor：** 安装后把包装放到官方 `agent` / `cursor-agent` 前面。发请求前按当前 Mode 强制换成映射模型（resume 也安全）。探索子代理的模型不改。

**Codex：** `codex` 包装会代理交互界面，按 Plan / Default 改写模型和力度（含 Shift+Tab）。`exec` / `review` / `mcp` 原样透传。代理起不来就直接跑官方 `codex`。

## 限制

- 安装或改配置之前已经开着的对话，必须重开。
- Cursor Agent 升级后如果补丁对不上，Cursor 侧自动切换会暂停（Agent 本身还能跑）。看 `status`。
- Codex 的远程连接方式仍是上游实验能力，协议变了会放行不拦截。
- 自更新只更新本工具，不会升级 Cursor Agent 或 Codex。
- `cursor agent` 会绕过包装。请直接用 `agent` 或 `cursor-agent`。

## 常见问题

**会改 Cursor 桌面版吗？**  
不会。只包装你 PATH 上的命令行入口。

**会不会盖掉我指定的 `--model`？**  
不会。那个参数只锁这一次对话，不改已保存的默认映射。

**装完 `status` 还是 inactive？**  
新开一个终端，让 PATH 吃到包装。Ubuntu 登录壳有时会在 `.bashrc` 之后又把 `~/.local/bin` 插到前面——再跑一次 `agent-auto-model install`，然后用 `bash -l -c 'agent-auto-model status'` 检查。

**Codex 界面还显示旧模型？**  
看 `~/.local/share/agent-auto-model/assets/codex-decisions.log`。改写成功后应出现映射后的 `model` / `effort`。改配置后请重启 TUI。

## License

[MIT](LICENSE)
