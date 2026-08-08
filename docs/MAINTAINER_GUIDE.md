# 维护指南

## Mode→模型挂钩

预加载脚本：`internal/assets/register.mjs`（经 `go:embed` 打进二进制，install/exec 时落到 `~/.local/share/cursor-mode-model/assets/`）。

锚点（Agent `2026.08.04-aaa8809` 核实）：

- `setCurrentModel` / `setCurrentModelWithParameters` / `setModelFromStoredId`（抓模型管理器 + 配置；后者调用时武装恢复后对齐）
- `getCurrentModel(){return this.deriveCurrentModelDetails()`（Mode 早于写模型时也能 stash）
- `setMetadata(e,t){this.metadataStore.set(e,t)}`（Mode 变化入口；并 stash 会话 store）
- `buildRequestedModel(){var e,t,r,n;const o=this.currentSelectedModel;`（**发送前强制**按当前 Mode 覆盖 `modelId`）

切模型必须走 `setModelFromStoredId(modelId, configProvider)`（或带第三参的 `setCurrentModelWithParameters`）；管理器上有 `configProvider`。只调两参会失败。`ok:false` 必须当失败并尝试降级 API，不能假装成功。

Mode 切换成功后还会写入会话 `lastUsedModel`，避免 resume 的 `model_restore` 用旧模型把选择打回去。

真机验收（两层都要看）：

1. `CURSOR_MODE_MODEL_DEBUG=1` 启动后，资产目录 `sync.log` 出现 `before_build` / `apply_result`。
2. 会话落盘里助手消息的 `providerOptions.cursor.modelName` 与 Mode 映射一致（不能只看界面选择器）。

升级 Cursor Agent 后先跑 `cursor-mode-model status`。锚点未命中则自动切换失效，但 Agent 仍可正常用（故障开放）。`status.active` 要求包装器真正在 PATH 上生效，不是只看配置开关。

## 包装与 PATH

`install` 在 `~/.local/share/cursor-mode-model/bin/` 写入 `agent` / `cursor-agent` 壳脚本，并尽量把该目录 prepend 进 `~/.zshrc` / `~/.bashrc`。`uninstall` 必须同时清掉这些 shell PATH 挂钩。官方入口仍在 `~/.local/share/cursor-agent/versions/<ver>/cursor-agent`；包装通过扫 versions 目录找最新一份，不依赖会被 `agent update` 改写的 `~/.local/bin` 符号链接。

shell **函数 / alias 优先于 PATH**：若用户或其它工具给 `agent` 定义了函数，包装器不会生效——`status` 的 `wrapper_effective` 会反映这一点。

## 与 pickup 的边界

本工具独立。即使用户同时装了 pickup 的命令拦截，只要包装目录在 PATH 更靠前，或 pickup 最终 `execvp("agent")` 走到本包装，Mode→模型就会生效。禁止把本逻辑并回 pickup 仓库。
