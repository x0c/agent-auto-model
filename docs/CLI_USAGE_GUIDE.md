# 命令面与 Mode 映射 Guide

## 文档定位

覆盖本工具的**用户命令**：查看状态、改 Mode→模型映射、推荐配置 vs 本地自定义、推荐表通配符默认、开关自动切换、会话锁定、卸载。

对外 [README.md](../README.md) / [README.zh-CN.md](../README.zh-CN.md) 的 **Usage / 用法** 节必须能让陌生人照着敲；本文与 README 命令清单保持一致，并多写 Agent 约束（禁止只答「支持」、禁止为回答用法翻源码）。

不覆盖：Cursor 预加载挂钩细节、Codex JSON-RPC 改写、PATH 包装实现、发版与 Homebrew。那些见 [MAINTAINER_GUIDE.md](MAINTAINER_GUIDE.md)。

## 机制定位

用户装好后，Cursor Agent CLI / Codex CLI 按 Mode 自动换模型。映射和总开关写在用户配置里，用 `agent-auto-model config …` 改。

## 默认映射

| Runtime | Mode | 默认模型 |
|---|---|---|
| Cursor | Plan | `claude-opus-*-thinking-high`（运行时展开为当前最新） |
| Cursor | Agent / Ask / Debug（配置键 `default` / `search` / `debug`） | `cursor-grok-*-high`（运行时展开为当前最新） |
| Codex | Plan | `gpt-*-sol:high`（运行时展开为当前最新 Sol） |
| Codex | Default | `gpt-*-terra:medium`（运行时展开为当前最新 Terra） |

Cursor 可配键：`plan` / `default` / `search` / `debug`。别名：`ask` → `search`，`agent` → `default`。CLI `--mode ask` 对应内部 `search`。

Codex 可配键：`plan` / `default`。取值是 `模型[:力度]`，例如 `gpt-*-sol:high`。

未写 runtime 前缀的 `config set plan …` 视为 `cursor.plan`（旧写法，Codex 必须带 `codex.`）。

默认 **模型映射来源** 是「推荐配置」：装完即用，并自动跟随 GitHub 仓库里的 `recommended-models.json`。自己改过映射的人升级后会判成「本地自定义」，不会被远程覆盖。

## 核心入口：回答用户时直接贴这些命令

### 看当前配置 / 是否生效

```bash
agent-auto-model status
agent-auto-model config show
```

`status` 里 `active=true` 表示包装已在 PATH 上生效。装完或改完配置后**已打开的对话要重开**。

### 改默认映射（会写入配置，之后一直生效）

```bash
# Cursor
agent-auto-model config set cursor.plan 'claude-opus-*-thinking-high'
agent-auto-model config set cursor.default 'cursor-grok-*-high'
agent-auto-model config set cursor.search 'cursor-grok-*-high'
agent-auto-model config set cursor.debug 'cursor-grok-*-high'

# Codex
agent-auto-model config set codex.plan 'gpt-*-sol:high'
agent-auto-model config set codex.default 'gpt-*-terra:medium'

# 一次改多条
agent-auto-model config set-many cursor.plan='claude-opus-*-thinking-high' cursor.default='cursor-grok-*-high' codex.plan='gpt-*-sol:high'
```

模型 ID 可用 `*` / `?` 通配符；发送前按当前可用模型展开，选最新版本。

**改映射会把来源切成「本地自定义」**，之后不再跟随仓库推荐。要重新跟随：

```bash
agent-auto-model config set-models-source recommended
```

切回来时如果本地映射和推荐不一致，命令会提示这会覆盖你改过的映射。

立刻拉一次仓库推荐（不改来源；来源仍是推荐配置时下次开对话才会用上）：

```bash
agent-auto-model config refresh-recommended
```

### 一键关闭 / 打开自动切换

```bash
# 两边一起关（只关映射，不卸包装）
agent-auto-model config disable

# 只关一边
agent-auto-model config disable --runtime cursor
agent-auto-model config disable --runtime codex

# 再打开
agent-auto-model config enable
agent-auto-model config enable --runtime cursor
```

`config disable` 与 `uninstall` 不同：前者关开关、映射还在；后者移除 PATH 包装，官方 CLI 不再被拦截。

环境变量总开关（当前进程）：`AGENT_AUTO_MODEL=0`。

### 只锁这一次会话（不改默认）

启动 Cursor / Codex 时显式指定模型，本次不再按 Mode 自动切：

```bash
agent --model <model-id> …
codex --model <model-id> …
# Codex 也认短参数：
codex -m <model-id> …
```

**AI 易错点**：Cursor 包装层目前只认 `--model` / `--model=`，不认单独的 `-m`。Codex 认 `--model`、`-m`、`-m=`。Codex TUI 里 `/model` 选了与映射不符的模型，同样会锁本会话。

### 其它配置

```bash
agent-auto-model config set-strict true|false          # Cursor：纠正失败则阻断发送
agent-auto-model config set-auto-update true|false
agent-auto-model config set-update-interval <hours>
agent-auto-model config set-models-source recommended|local
agent-auto-model config refresh-recommended
agent-auto-model config reset                          # 恢复推荐配置与开关
agent-auto-model uninstall                             # 卸包装
```

非 TTY 或加 `--json` 时输出 JSON。

## 使用约束

- **AI 易错点** 【禁止】用户问「是否支持关闭 / 改默认模型 / 命令是啥」时只答「支持」→ 必须从本文复制完整命令。全局「面向用户不要抛命令参数」**不适用于本项目的用法问答**。
- **AI 易错点** 【禁止】为了回答上述用法去读 `internal/app` / `internal/config`。本文与 `config show` 已足够。
- 【隐性依赖】改映射 / 开关 / 安装之后，已经开着的 Agent 会话必须重启才生效。
- 【消歧】`config disable` = 关自动切换；`uninstall` = 拿掉包装。不要把「关掉」说成卸载。
- 【消歧】`--model` 锁的是**这一次会话**，不改默认映射；会话锁定可以用具体版本号。改默认只能用 `config set`。推荐表 / 出厂默认必须用通配符，禁止钉死 Opus / Sol / Terra / Grok 的版本号。
- 【消歧】`config set` 会把映射来源切成本地自定义；跟随仓库推荐要用 `config set-models-source recommended`。来源是整表两态，不做「改过的键保留、没改的继续跟随」。
- 【叫法统一】产品名 `agent-auto-model`。
- 【隐式语义】总开关 `enabled=false` 时，单个 runtime 即使仍是 enabled 也不再切换（`RuntimeEnabled` 要求两层都开）。

## 验证路径

```bash
agent-auto-model config set cursor.plan 'claude-opus-*-thinking-high'
agent-auto-model config show
agent-auto-model config disable
agent-auto-model status
agent-auto-model config enable
```

期望：`show` 里出现刚写入的映射；`disable` 后 `status` 不再把自动切换标成启用中。真机 Mode 切换验收见 [MAINTAINER_GUIDE.md](MAINTAINER_GUIDE.md)。

改命令面代码后：

```bash
go test ./internal/app ./internal/config ./internal/recommended ./internal/wrap
```

## 领域引用

- [MAINTAINER_GUIDE.md](MAINTAINER_GUIDE.md)：Cursor 发送前强制、Codex 代理改写、PATH 包装、自更新、发版。改那些实现时联读，不要把命令面细节再抄一遍。

## 待补充

- Cursor 包装是否应同样识别 `-m`（README 曾把 `-m` 写成两端通用；代码上只有 Codex 认短参数）。来源：2026-08-14 对照 `internal/wrap` 与 `internal/runtime/codex/find.go`。

<!-- 该文档由 doc-init 生成于 2026-08-14；定位：Agent 回答用法 / 改命令面与配置前的快速参考 -->
