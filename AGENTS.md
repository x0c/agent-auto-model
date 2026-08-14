<!-- managed:inherited-agents:start -->
<!-- source: /Users/geraltgraham/Codes/cursor-mode-model/AGENTS.md -->
# agent-auto-model

独立工具：让 Cursor Agent CLI / Codex CLI 在会话 Mode 变化时自动切换模型。

通用工程规范：[Go 规范](../_standards/go.md)

## 交付闭环（强制）

改完 Bug / 加完功能后，**自测通过即立刻发版**，不要等用户再说「发布一下」：

1. 跑通本仓库验证命令（见 [cli/AGENTS.md](cli/AGENTS.md)）
2. 按 SemVer 升版本、提交、打 `vX.Y.Z` tag
3. 推双远端并执行 `cli/scripts/publish-release.sh`
4. 回报：版本号、Release URL、Homebrew 配方是否已跟上

未完成发版不得宣称任务结束。仅文档笔误、纯本地试验、或用户明确说「先别发」时可跳过。

## 文档导航

- [cli/AGENTS.md](cli/AGENTS.md)：改、评审或发布本 CLI 前必读（含双远端与发版命令）。
- 公开仓库：`https://github.com/x0c/cursor-mode-model`
- Forgejo 备份：`ssh://git@10.10.10.2:2222/Max/cursor-mode-model.git`

## 组件一览

| 目录 | 技术栈 | 状态 |
|---|---|---|
| `cli/` | Go | 活跃 |

<!-- managed:inherited-agents:end -->

# agent-auto-model CLI 规范

## 文档导航

- `README.md` / `README.zh-CN.md`：对外安装、用法、FAQ（公开面零私有基础设施）
- `docs/MAINTAINER_GUIDE.md`：改预加载挂钩、Codex 代理、包装安装、Ubuntu login PATH、Grok fast 参数、锚点漂移排障、双远端发版前必读
- `scripts/publish-release.sh`：本机发版收尾（Release 附件 + Homebrew 配方，防回退）
- `scripts/bump-homebrew-formula.py`：配方 url/sha256 写入与版本回退防护

## 架构约束

- 外层：PATH 前置包装（`~/.local/share/agent-auto-model/bin/{agent,cursor-agent,codex}`，Windows 为 `.cmd`）。
- Cursor 内层：Node `registerHooks` 改写 Agent 打包 JS。锚点唯一来源：`internal/assets/anchors.json`。
- Codex 内层：进程内 UDS WebSocket 代理，上游 `codex app-server --stdio`，TUI 以 `codex --remote unix://<临时 sock>` 连接；改写 `turn/start` / `thread/settings/update` 的 `model`+`effort`（及 `collaborationMode.settings`）。JSON-RPC 字段集中在 `internal/runtime/codex/rewrite`。
- 配置 v2：`runtimes.cursor` / `runtimes.codex`；v1 扁平 `models` 读入时视为 Cursor 映射。
- 显式 `--model` / `-m`：本会话不自动切换。
- 总开关：`AGENT_AUTO_MODEL=0`（兼容 `CURSOR_MODE_MODEL=0`）或 `config disable`。
- Cursor 锚点漂移 / Codex 代理失败：故障开放。
- 二进制名：`agent-auto-model`；`cursor-mode-model` 为过渡别名。Go module 仍为 `github.com/x0c/cursor-mode-model`（与公开 GitHub 仓库一致，保证 `go install`）。
- 配置/数据目录：`~/.config/agent-auto-model`、`~/.local/share/agent-auto-model`；读配置时回退旧 `cursor-mode-model` 路径。

## Remote

| 名称 | URL | 用途 |
|---|---|---|
| `origin` | `ssh://git@10.10.10.2:2222/Max/cursor-mode-model.git` | Forgejo 备份 |
| `github` | `git@github.com:x0c/cursor-mode-model.git` | 公开协作 / Release / Homebrew 源 |

## 发版要求

**交付闭环（强制）**：功能 / 修复自测通过后立刻发版，**不要等用户再 trigger**。未发版不得收工。

自测通过后按 SemVer 升 `VERSION` 与 `main.version`，提交后：

```bash
git tag vX.Y.Z
git push origin main --tags
git push github main --tags
bash scripts/publish-release.sh vX.Y.Z
```

公开安装渠道：Homebrew `x0c/tap/agent-auto-model`（`x0c/tap/cursor-mode-model` 为别名配方）、`install.sh`、`install.ps1`、`go install`。  
CI 的 `release.yml` 只补其它平台包与配方兜底，**不得**作为唯一升级通路。

用户侧升级：

```bash
brew upgrade x0c/tap/agent-auto-model && agent-auto-model install
# 或
curl -fsSL https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.sh | bash
```

## 验证要求

```bash
go test ./...
node --test internal/assets/register.test.mjs
go build -ldflags "-X main.version=$(tr -d '[:space:]' < VERSION)" -o /tmp/agent-auto-model ./cmd/agent-auto-model
go build -ldflags "-X main.version=$(tr -d '[:space:]' < VERSION)" -o /tmp/cursor-mode-model ./cmd/cursor-mode-model
```
