# 维护指南

## Mode→模型挂钩

预加载脚本：`internal/assets/register.mjs`（经 `go:embed` 打进二进制，install/exec 时落到 `~/.local/share/cursor-mode-model/assets/`）。

**锚点字符串唯一来源**：`internal/assets/anchors.json`（Go 的 `anchors.Check` 与预加载 `patchSource` 都读它；禁止在两处各写一份）。

挂钩点（Agent `2026.08.04-aaa8809` 核实）：

- `setCurrentModel` / `setCurrentModelWithParameters` / `setModelFromStoredId`（抓模型管理器 + 配置；后者调用时武装恢复后对齐）
- `getCurrentModel(){return this.deriveCurrentModelDetails()`（Mode 早于写模型时也能 stash）
- `setMetadata(e,t){this.metadataStore.set(e,t)}`（Mode 变化入口；stash 会话 store，并 `subscribeToMetadata('mode')`）
- `buildRequestedModel(){var e,t,r,n;const o=this.currentSelectedModel;`（**发送前强制**按当前 Mode 覆盖 `modelId`）

### 模式识别（P0）

- **权威值**：`store.getMetadata('mode')`（resume 时元数据从磁盘加载，不一定再触发 `setMetadata`）。
- **事件缓存**：`setMetadata` / 订阅回调写入 `__cursorModeModelLastMode`。
- **禁止瞎猜**：两者都没有时**不强制**（旧版回退 `default` 会把 Plan resume 压成 Grok，属回归）。

### 模型归一

`claude-opus-5-thinking-high` 经官方 `mapModelToParameterizedSelection` 会变成 `claude-opus-5` + thinking/effort 参数。比较与强制必须用归一后的 id，并保留 `getParametersForModel` 参数，否则会悄悄降档。

### 发送前强制与严格模式

- 默认：发送前纠正内存选择，并异步走官方 API；失败只告警。
- 配置 `"strict": true`：纠正后仍不等价则**抛错阻断本轮发送**。
- 审计日志：`~/.local/share/cursor-mode-model/assets/decisions.log`（约 1MB 轮转，只记决策元数据）。

切模型必须走 `setModelFromStoredId(modelId, configProvider)`（或带第三参的 `setCurrentModelWithParameters`）。`ok:false` 必须当失败并尝试降级 API。成功后 `notifyListeners()` + 写 `lastUsedModel`（归一后的真实 id）。

### 明确不做

子任务 / 探索子代理（`exploreSubagentModel`）保持 Cursor 原样，不按模式绑定。

## 真机验收

两层都要看：

1. `decisions.log` /（调试时）`sync.log` 出现 `before_build` / `corrected` / `apply_done`。
2. 会话落盘 `providerOptions.cursor.modelName` 与 meta `lastUsedModel` 与 Mode 映射一致（**不能只看界面**）。

**必须含 resume 用例**（不带 `--mode`）：

```bash
export PATH="$HOME/.local/share/cursor-mode-model/bin:$PATH"
# 新建 Plan
agent --mode plan --print --force '只回复：PLAN_OK'
# 记下 chat id，然后 resume 且不带 --mode —— 仍须 Opus
agent --resume <id> --print --force '只回复：PLAN_RESUME_OK'
```

CLI `--mode` 只接受 `plan` / `ask`（Ask 内部是 `search`）；Agent 模式不要传 `--mode`。

注意：同一会话 id 在不同工作目录 resume 时，落盘可能写到另一份 `~/.cursor/chats/<hash>/<id>/`。

升级 Cursor Agent 后先跑 `cursor-mode-model status`。`status.active` 要求包装器真正在 PATH 上生效。长期挂着的旧会话需**重启**才会吃到新挂钩。

## 包装与 PATH

`install` 在 `~/.local/share/cursor-mode-model/bin/` 写入包装脚本（Unix）或 `.cmd`（Windows），并尽量 prepend 进 shell rc / 用户 PATH；`uninstall` 必须对称清理。官方入口通常在 `~/.local/share/cursor-agent/versions/<ver>/cursor-agent`（Windows 也可能在 `%LOCALAPPDATA%\cursor-agent\versions\`）。

shell **函数 / alias 优先于 PATH**——`status.wrapper_effective` 会反映。

## 发版与双远端

- 公开协作 / Release / Homebrew 源：GitHub `x0c/cursor-mode-model`
- 备份：Forgejo（见根/CLI `AGENTS.md` Remote 表）
- 本机收尾：`bash scripts/publish-release.sh vX.Y.Z`（不等 CI 排队）
- 配方仓库：复用 `x0c/homebrew-tap`，写入带版本回退防护

发版踩坑（必守）：

1. **新建配方必须先 `git add` 再看 staged diff**。对未跟踪文件跑 `git diff --quiet` 会误判「无需改动」，配方看起来更新成功、远端却没有文件。
2. **本机上传附件与 CI `release.yml` 可能抢同名资源导致 404**。附件失败不要中断配方更新；用户能否 `brew upgrade` 取决于配方指向的源码归档，不取决于预编译包是否齐。
3. **v* tag 推到 github 后立刻跑本机 `publish-release.sh`**；CI 只补其它平台包与配方兜底，不能当唯一升级通路。

## 独立性

本仓库保持独立产品边界，不并入其它会话托管 / Agent 启动链工具。
