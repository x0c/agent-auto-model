<!-- managed:inherited-agents:start -->
<!-- source: /Users/geraltgraham/Codes/agent-auto-model/AGENTS.md -->
# agent-auto-model

独立工具：让 Cursor Agent CLI / Codex CLI 在会话 Mode 变化时自动切换模型。

通用工程规范：[Go 规范](../_standards/go.md)

Git 仓库与源码在 [cli/](cli/)。改代码、发版、跑测试以 [cli/AGENTS.md](cli/AGENTS.md) 为准。

## 回答用法问题（强制）

用户问本工具「是不是 CLI」「是否支持关闭」「怎么改默认模型」「命令是啥」时：

1. 先读 [cli/docs/CLI_USAGE_GUIDE.md](cli/docs/CLI_USAGE_GUIDE.md) 或 [cli/README.zh-CN.md](cli/README.zh-CN.md) 的「用法」节，**不要翻源码**。
2. **直接给出完整可复制命令**，禁止只答「支持」。
3. 本项目就是命令行工具，用法问答必须贴命令；全局「面向用户不要抛命令参数」**不适用于本项目的用法问答**。

## 交付闭环（强制）

改完 Bug / 加完功能后，**自测通过即立刻发版**，不要等用户再说「发布一下」：

1. 跑通本仓库验证命令（见 [cli/AGENTS.md](cli/AGENTS.md)）
2. 按 SemVer 升版本、提交、打 `vX.Y.Z` tag
3. 推双远端并执行 `cli/scripts/publish-release.sh`
4. 回报：版本号、Release URL、Homebrew 配方是否已跟上

未完成发版不得宣称任务结束。仅文档笔误、纯本地试验、或用户明确说「先别发」时可跳过。

## 文档导航

> 以下文档在涉及对应领域的开发、评审、排查或回答用户用法时先读取。

- [cli/docs/CLI_USAGE_GUIDE.md](cli/docs/CLI_USAGE_GUIDE.md)：怎么用、改 Mode→模型映射、推荐配置 vs 本地自定义、一键关闭/打开自动切换、会话锁定、`config` 子命令。
- [cli/docs/MAINTAINER_GUIDE.md](cli/docs/MAINTAINER_GUIDE.md)：Cursor 挂钩、Codex 代理、PATH 包装、自更新、双远端发版、锚点漂移、推荐模型配置。
- [cli/AGENTS.md](cli/AGENTS.md)：改、评审或发布本 CLI（验证命令、Remote、交付闭环）。
- [cli/README.zh-CN.md](cli/README.zh-CN.md) / [cli/README.md](cli/README.md)：对外安装与用法（改映射、跟随推荐、关闭自动切换、会话锁定）。
- 公开仓库：`https://github.com/x0c/agent-auto-model`
- Forgejo 备份：`ssh://git@10.10.10.2:2222/Max/agent-auto-model.git`

## 组件一览

| 目录 | 技术栈 | 状态 |
|---|---|---|
| `cli/` | Go | 活跃 |

<!-- managed:inherited-agents:end -->

# agent-auto-model CLI 规范

## 回答用法问题（强制）

用户问「是否支持关闭 / 怎么改默认模型 / 命令是啥」时：先读 `docs/CLI_USAGE_GUIDE.md` 或 README 的 Usage / 用法节，直接贴完整命令。禁止只答「支持」，禁止为回答用法去翻 `internal/app` / `internal/config`。本项目用法问答必须贴命令。

## 文档导航

> 以下文档在涉及对应领域的开发、评审、排查或回答用户用法时先读取。

- `docs/CLI_USAGE_GUIDE.md`：怎么用、改 Mode→模型映射、推荐配置 vs 本地自定义、一键关闭/打开自动切换、会话锁定、`config` 子命令
- `README.md` / `README.zh-CN.md`：对外安装与用法（改映射、跟随推荐、关闭自动切换、会话锁定）；与 CLI_USAGE_GUIDE 命令清单对齐
- `docs/MAINTAINER_GUIDE.md`：Cursor 挂钩、Codex 代理、PATH 包装、Ubuntu login PATH、Grok fast、锚点漂移、自更新、双远端发版
- `scripts/publish-release.sh`：本机发版收尾（Release 附件 + Homebrew 配方，防回退）
- `scripts/bump-homebrew-formula.py`：配方 url/sha256 写入与版本回退防护

## 架构约束

- 外层：PATH 前置包装（`~/.local/share/agent-auto-model/bin/{agent,cursor-agent,codex}`，Windows 为 `.cmd`）。
- Cursor 内层：Node `registerHooks` 改写 Agent 打包 JS。锚点唯一来源：`internal/assets/anchors.json`。
- Codex 内层：进程内 UDS WebSocket 代理，上游 `codex app-server --stdio`，TUI 以 `codex --remote unix://<临时 sock>` 连接；改写 `turn/start` / `thread/settings/update` 的 `model`+`effort`（及 `collaborationMode.settings`）。JSON-RPC 字段集中在 `internal/runtime/codex/rewrite`。
- 配置 v2：`runtimes.cursor` / `runtimes.codex`；v1 扁平 `models` 读入时视为 Cursor 映射。
- 模型映射来源：`models_source=recommended|local`。缺省按内容推断（与内置推荐一致→推荐配置，否则本地自定义）。推荐文件：`recommended-models.json`，后台 ETag 刷新。`config set` 后切成本地自定义。切回：`config set-models-source recommended`。推荐表禁止钉死版本号（Opus / Grok / Sol / Terra 用通配符）。
- 显式 `--model`：Cursor 本会话不自动切换（包装只认 `--model` / `--model=`）。Codex 还认 `-m` / `-m=`。
- 总开关：`AGENT_AUTO_MODEL=0` 或 `config disable`。
- Cursor 锚点漂移 / Codex 代理失败：故障开放。
- 二进制名：`agent-auto-model`。Go module：`github.com/x0c/agent-auto-model`。
- 配置/数据目录：`~/.config/agent-auto-model`、`~/.local/share/agent-auto-model`。安装时若发现旧配置目录会一次性迁走，并清掉本机残留的旧命令名。

## Remote

| 名称 | URL | 用途 |
|---|---|---|
| `origin` | `ssh://git@10.10.10.2:2222/Max/agent-auto-model.git` | Forgejo 备份 |
| `github` | `git@github.com:x0c/agent-auto-model.git` | 公开协作 / Release / Homebrew 源 |

## 发版要求

**交付闭环（强制）**：功能 / 修复自测通过后立刻发版，**不要等用户再 trigger**。未发版不得收工。

自测通过后按 SemVer 升 `VERSION` 与 `main.version`，提交后：

```bash
git tag vX.Y.Z
git push origin main --tags
git push github main --tags
bash scripts/publish-release.sh vX.Y.Z
```

公开安装渠道：Homebrew `x0c/tap/agent-auto-model`、`install.sh`、`install.ps1`、`go install`。  
CI 的 `release.yml` 只补其它平台包与配方兜底，**不得**作为唯一升级通路。

用户侧升级：

```bash
brew upgrade x0c/tap/agent-auto-model && agent-auto-model install
# 或
curl -fsSL https://raw.githubusercontent.com/x0c/agent-auto-model/main/install.sh | bash
```

## 验证要求

```bash
go test ./...
node --test internal/assets/register.test.mjs
go build -ldflags "-X main.version=$(tr -d '[:space:]' < VERSION)" -o /tmp/agent-auto-model ./cmd/agent-auto-model
```

## 领域地图（doc-init）

<!-- 覆盖度复核基线：2026-08-14 · 源码指纹 扫描 59 文件 / Go 32 · Ruby 2 · Python 1 / 0 子模块 · 基线提交 e1c135d -->

| 领域 | 入口锚点 |
|------|---------|
| 命令面与 Mode 映射 | internal/app/ · internal/config/ · cmd/agent-auto-model/ · internal/status/ |
| 仓库推荐映射 | internal/recommended/ · recommended-models.json |
| Cursor 挂钩 | internal/assets/ · internal/anchors/ |
| Codex 代理 | internal/runtime/ · internal/runtime/codex/ |
| 安装与 PATH 包装 | internal/install/ · internal/wrap/ · internal/agentbin/ · internal/paths/ |
| 发版与自更新 | scripts/ · internal/autoupdate/ · Formula/ |
