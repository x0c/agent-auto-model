<!-- managed:inherited-agents:start -->
<!-- source: /Users/geraltgraham/Codes/cursor-mode-model/AGENTS.md -->
# cursor-mode-model

独立工具：让 Cursor Agent CLI 在会话 Mode 变化时自动切换模型。

通用工程规范：[Go 规范](../_standards/go.md)

## 文档导航

- [cli/AGENTS.md](cli/AGENTS.md)：改、评审或发布本 CLI 前必读（含双远端与发版命令）。
- 公开仓库：`https://github.com/x0c/cursor-mode-model`
- Forgejo 备份：`ssh://git@10.10.10.2:2222/Max/cursor-mode-model.git`

## 组件一览

| 目录 | 技术栈 | 状态 |
|---|---|---|
| `cli/` | Go | 活跃 |

<!-- managed:inherited-agents:end -->

# cursor-mode-model CLI 规范

## 文档导航

- `README.md` / `README.zh-CN.md`：对外安装、用法、FAQ（公开面零私有基础设施）
- `docs/MAINTAINER_GUIDE.md`：改预加载挂钩、包装安装、锚点漂移排障、双远端发版前必读
- `scripts/publish-release.sh`：本机发版收尾（Release 附件 + Homebrew 配方，防回退）
- `scripts/bump-homebrew-formula.py`：配方 url/sha256 写入与版本回退防护

## 架构约束

- 外层：PATH 前置包装（`~/.local/share/cursor-mode-model/bin/{agent,cursor-agent}`，Windows 为 `.cmd`）注入 `NODE_OPTIONS=--import=…` 后转调官方 Agent。
- 内层：Node `registerHooks` 改写 Agent 打包 JS：模式以会话 `getMetadata('mode')` 为准（含 resume）；`buildRequestedModel` 发送前强制；模型 id 归一比较；成功后 `notifyListeners` + 写 `lastUsedModel`；默认写 `decisions.log`；`strict` 可阻断发送。
- 锚点字符串唯一来源：`internal/assets/anchors.json`。
- 显式 `--model`：仍可注入预加载，但设置 `CURSOR_MODE_MODEL_LOCK=1`，本会话不自动切换。
- 总开关：`CURSOR_MODE_MODEL=0` 或配置 `enabled: false` / `config disable`。
- 锚点漂移：故障开放（不阻断 Agent）；`status` 可诊断。
- module 路径：`github.com/x0c/cursor-mode-model`（公开协作以 GitHub 为准）。

## Remote

| 名称 | URL | 用途 |
|---|---|---|
| `origin` | `ssh://git@10.10.10.2:2222/Max/cursor-mode-model.git` | Forgejo 备份 |
| `github` | `git@github.com:x0c/cursor-mode-model.git` | 公开协作 / Release / Homebrew 源 |

## 发版要求

功能/修复验证通过后按 SemVer 升 `VERSION` 与 `main.version`，提交后：

```bash
git tag vX.Y.Z
git push origin main --tags
git push github main --tags
bash scripts/publish-release.sh vX.Y.Z
```

公开安装渠道：Homebrew `x0c/tap/cursor-mode-model`、`install.sh`、`install.ps1`、`go install`。  
CI 的 `release.yml` 只补其它平台包与配方兜底，**不得**作为唯一升级通路。

## 验证要求

```bash
go test ./...
node --test internal/assets/register.test.mjs
go build -ldflags "-X main.version=$(tr -d '[:space:]' < VERSION)" -o /tmp/cursor-mode-model ./cmd/cursor-mode-model
```
