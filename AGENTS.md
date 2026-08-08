<!-- managed:inherited-agents:start -->
<!-- source: /Users/geraltgraham/Codes/cursor-mode-model/AGENTS.md -->
# cursor-mode-model

独立工具：让 Cursor Agent CLI 在会话 Mode 变化时自动切换模型。与 pickup 无关、不依赖 pickup。

通用工程规范：[Go 规范](../_standards/go.md)

## 文档导航

- [cli/AGENTS.md](cli/AGENTS.md)：改、评审或发布本 CLI 前必读。Remote：`ssh://git@10.10.10.2:2222/Max/cursor-mode-model.git`

## 组件一览

| 目录 | 技术栈 | 状态 |
|---|---|---|
| `cli/` | Go | 活跃 |

<!-- managed:inherited-agents:end -->

# cursor-mode-model CLI 规范

## 文档导航

- `README.md`：安装、用法、Mode→模型映射与排障入口
- `docs/MAINTAINER_GUIDE.md`：改预加载挂钩、包装安装、锚点漂移排障前必读

## 架构约束

- **与 pickup 完全解耦**：不得 import、调用或依赖 pickup；不得把本能力塞进 pickup 启动链。
- 外层：PATH 前置包装（`~/.local/share/cursor-mode-model/bin/{agent,cursor-agent}`）注入 `NODE_OPTIONS=--import=…` 后 `exec` 官方 Agent。
- 内层：Node `registerHooks` 改写 Agent 打包 JS：`setMetadata("mode", …)` 后切模型；`buildRequestedModel` 发送前再按 Mode 强制 `modelId`；成功后写 `lastUsedModel`，并对 resume `model_restore` 做延迟对齐。
- 显式 `--model`：仍可注入预加载，但设置 `CURSOR_MODE_MODEL_LOCK=1`，本会话不自动切换。
- 总开关：`CURSOR_MODE_MODEL=0` 或配置 `enabled: false`。
- 锚点漂移：故障开放（不阻断 Agent）；`status` 可诊断。

## 发版要求

功能/修复验证通过后按 SemVer 升版本，提交、打 tag、推送 Forgejo；本机 `go install` 覆盖安装。

## 验证要求

```bash
go test -race ./...
go build -o /tmp/cursor-mode-model ./cmd/cursor-mode-model
```
