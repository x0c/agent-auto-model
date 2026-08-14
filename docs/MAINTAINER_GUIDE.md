# 维护指南

命令面（`config set` / `disable` / 推荐来源 / 会话锁定）见 [CLI_USAGE_GUIDE.md](CLI_USAGE_GUIDE.md)，本文只覆盖挂钩实现、包装、自更新、推荐配置拉取、发版与双机对齐。

## Mode→模型挂钩

预加载脚本：`internal/assets/register.mjs`（经 `go:embed` 打进二进制，install/exec 时落到 `~/.local/share/agent-auto-model/assets/`）。

**锚点字符串唯一来源**：`internal/assets/anchors.json`（Go 的 `anchors.Check` 与预加载 `patchSource` 都读它；禁止在两处各写一份）。

挂钩点（Agent `2026.08.11-e8db854` 核实；minify 局部变量名会漂，以 `anchors.json` 为准）：

- `setCurrentModel` / `setCurrentModelWithParameters` / `setModelFromStoredId`（抓模型管理器 + 配置；后者调用时武装恢复后对齐）
- `getCurrentModel(){return this.deriveCurrentModelDetails()`（Mode 早于写模型时也能 stash）
- `setMetadata(e,t){this.metadataStore.set(e,t)}`（Mode 变化入口；stash 会话 store，并 `subscribeToMetadata('mode')`）
- `subscribe(e){return this.listeners.add(e),e(this.getCurrentModel()),()=>{this.listeners.delete(e)}}`（抓住 TUI 底栏订阅，Mode 变化时同步推模型，避免只改 Mode 标签、底栏仍显示旧模型）
- `buildRequestedModel(){var e,t,n,r;const o=this.currentSelectedModel;`（**发送前强制**按当前 Mode 覆盖 `modelId`）

### 模式识别（P0）

- **权威值**：`store.getMetadata('mode')`（resume 时元数据从磁盘加载，不一定再触发 `setMetadata`）。
- **事件缓存**：`setMetadata` / 订阅回调写入 `__cursorModeModelLastMode`。
- **禁止瞎猜**：两者都没有时**不强制**（旧版回退 `default` 会把 Plan resume 压成 Grok，属回归）。

### 模型归一

`claude-opus-5-thinking-high` 经官方 `mapModelToParameterizedSelection` 会变成 `claude-opus-5` + thinking/effort 参数。比较与强制必须用归一后的 id，并保留 `getParametersForModel` 参数，否则会悄悄降档。

Grok 4.6 起：`cursor-grok-*-high` 与 `cursor-grok-*-high-fast` **共用** `modelId=grok-4.6`，只靠参数 `fast` 区分。官方 `getParametersForModel` 会沿用上次选择 / `cli-config.json` 里存的 `fast=true`，导致通配符选中 non-fast 别名后 UI 仍显示 High Fast。强制与等价判断必须按规格意图覆盖并比较 `fast`（`*-high` → `false`，`*-high-fast` → `true`）。

### 通配符模型

配置可写 `cursor-grok-*-high` 这类 shell 通配符。预加载在发送前用 `parameterizedModelMap` / `availableModels` 收集候选，按版本号选最新；同版本优先非 `-fast`。展开失败时故障开放（本轮不强制），并写 `glob_expand_miss` 诊断。

### 发送前强制与严格模式

- 默认：发送前纠正内存选择，并异步走官方 API；失败只告警。
- 配置 `"strict": true`：纠正后仍不等价则**抛错阻断本轮发送**。
- 审计日志：`~/.local/share/agent-auto-model/assets/decisions.log`（约 1MB 轮转，只记决策元数据）。

切模型必须走 `setModelFromStoredId(modelId, configProvider)`（或带第三参的 `setCurrentModelWithParameters`）。`ok:false` 必须当失败并尝试降级 API。成功后 `notifyListeners()` + 写 `lastUsedModel`（归一后的真实 id）。

### 明确不做

子任务 / 探索子代理（`exploreSubagentModel`）保持 Cursor 原样，不按模式绑定。

## 真机验收

两层都要看：

1. `decisions.log` /（调试时）`sync.log` 出现 `before_build` / `corrected` / `apply_done`。
2. 会话落盘 `providerOptions.cursor.modelName` 与 meta `lastUsedModel` 与 Mode 映射一致（**不能只看界面**）。
3. Grok 通配符 `cursor-grok-*-high`：decisions.log 里应有 `"fast":"false"`（或字段缺失且 UI 非 High Fast）；仅 modelId 同为 `grok-4.6` 不够——High 与 High Fast 共用 id，靠参数区分。若 UI 仍显示 High Fast，查 `~/.cursor/cli-config.json` 的 `modelParameters.grok-4.6` / `selectedModel.parameters` 是否残留 `fast: true`。

**必须含 resume 用例**（不带 `--mode`）：

```bash
export PATH="$HOME/.local/share/agent-auto-model/bin:$PATH"
# 新建 Plan
agent --mode plan --print --force '只回复：PLAN_OK'
# 记下 chat id，然后 resume 且不带 --mode —— 仍须 Opus
agent --resume <id> --print --force '只回复：PLAN_RESUME_OK'
```

CLI `--mode` 只接受 `plan` / `ask`（Ask 内部是 `search`）；Agent 模式不要传 `--mode`。

注意：同一会话 id 在不同工作目录 resume 时，落盘可能写到另一份 `~/.cursor/chats/<hash>/<id>/`。

升级 Cursor Agent 后先跑 `agent-auto-model status`。`status.active` 要求至少一个 runtime 的包装器真正在 PATH 上生效。长期挂着的旧会话需**重启**才会吃到新挂钩。

## Codex 代理

交互式 TUI 不打补丁（Rust 原生二进制），而是：

1. 包装器在临时 UDS 上做 WebSocket 服务端（TUI 握手路径为 `GET /rpc`）。
2. 上游拉起官方 `codex app-server --stdio`。
3. TUI 以 `codex --remote unix://<sock>` 连接。
4. 改写 `thread/start`、`thread/resume`、`thread/settings/update`、`turn/start`：按 `collaborationMode.mode`（`plan` / `default`）写入 `model` + `effort`，并同步 `collaborationMode.settings`。
5. `initialize` 若缺少 `experimentalApi`，代理会补上（否则 `collaborationMode` 字段会被服务端拒绝）。
6. 用户在 TUI `/model` 显式选了与映射不符的模型 → 本会话上锁。CLI `-m/--model` 同样上锁。
7. `exec` / `review` / `mcp` 等子命令、以及已带 `--remote` 的调用原样透传。
8. 代理起不来：stderr 一行告警后 exec 官方 `codex`。

改写字段集中在 `internal/runtime/codex/rewrite`。审计：`~/.local/share/agent-auto-model/assets/codex-decisions.log`。

验收：`status` 里 `[codex] wrapper=true`，Shift+Tab 切 Plan 后 log 出现 `ev=corrected mode=plan`，模型展开为当前最新 Sol。

## 包装与 PATH

`install` 在 `~/.local/share/agent-auto-model/bin/` 写入包装脚本（Unix）或 `.cmd`（Windows）：`agent`、`cursor-agent`、`codex`。安装时若发现旧配置目录会一次性迁走，并清掉本机残留的旧包装目录与旧命令名，避免旧入口仍排在 PATH 最前。

**hook 范围**：`.zshrc`、`.bashrc`、`.zprofile`、**`.profile`**。标记行为 `# agent-auto-model PATH`（安装/卸载时同时清掉本机残留的旧 PATH 标记）。

**验收 login shell**：用用户真正的默认 shell，不要想当然用 `bash -l`。这台 Mac 默认是 zsh；若存在独立 `.bash_profile` 且不 source `.profile`，`bash -l` 会看不到包装，从而误判「没装上」。

```bash
# Mac（zsh）
zsh -lic 'agent-auto-model status'       # wrapper_effective / active 应为 true
zsh -lic 'type -a agent | head -1'       # 应命中 ~/.local/share/agent-auto-model/bin/agent
# 开发机 login bash（.profile 末尾有 hook）
bash -l -c 'agent-auto-model status'
```

`status` 里的包装路径必须是新目录。只看 `version` 不够：改名后旧包装目录仍可能排在 PATH 最前，命令能跑但自动切换没生效。`install` 必须清掉旧包装目录；若仍命中旧路径，先删旧 `bin/` 再装一次，并**新开终端**。

**入口别走岔路**：

- 请直接敲 `agent` 或 `cursor-agent`（经 PATH 包装）。
- **`cursor agent` 不行**：官方 `~/.local/bin/cursor` shim 在无桌面 IDE 时会 `exec ~/.local/bin/agent`，硬编码绕过包装目录。
- shell **函数 / alias 优先于 PATH**（如 pickup 的 `cursor-agent()`）——`status.wrapper_effective` 会反映；管理类子命令（`login` / `update` / `mcp` …）通常 passthrough 到真二进制。

## 静默自更新

- 入口：包装路径拉后台子进程 `agent-auto-model update --auto --quiet`。`status` / `config` 同样只踢后台，避免诊断命令卡在 GitHub。显式 `update`（可加 `--force`）才同步检查。
- 更新源：固定走 GitHub Releases latest API，按 `agent-auto-model_<version>_<os>_<arch>.{tar.gz|zip}` 选当前平台资产。请求带 `User-Agent: agent-auto-model`，HTTP 超时 30s。
- 查询失败后 15 分钟再试，不要把一次超时当成整段检查间隔（默认 24h）冷却；`status` 里的「自更新错误」会留到下次成功。立刻清掉：`agent-auto-model update --force`。
- 安装收尾：下载并替换二进制后，必须复用 `install.Install(...)` 刷新 wrapper、运行时资产和 PATH 片段。
- 运行态文件：`~/.local/share/agent-auto-model/autoupdate.json`。
- 测试时可用 `AGENT_AUTO_MODEL_UPDATE_LATEST_URL` 覆盖 latest API。

## 推荐模型配置

- 权威文件：仓库根 `recommended-models.json`（与 `internal/recommended/recommended-models.json` 必须保持一致，单测对照）。
- **禁止钉死版本号**：Opus / Grok / Sol / Terra 一律通配符（如 `claude-opus-*-thinking-high`、`gpt-*-sol:high`），运行时按当前可用模型选最新。
- 客户端默认拉 `https://raw.githubusercontent.com/x0c/agent-auto-model/main/recommended-models.json`；测试用 `AGENT_AUTO_MODEL_RECOMMENDED_URL` 覆盖。
- 检测：后台 `update --auto` 顺带做 ETag 条件请求，间隔约 6 小时；失败保留旧缓存，再没有则用内置副本。不要绑发版。
- 运行态：`~/.local/share/agent-auto-model/recommended.json`。
- 生效：`models_source=recommended` 时用缓存覆盖模型映射；`local` 时认用户文件。`config set` 会切到 `local`。来源是整表两态，不做「改过的键保留、没改的继续跟随」。
- 改完推荐表并推到 `main` 后，跟随推荐的机器跑 `agent-auto-model config refresh-recommended` 即可吃到，不必等二进制升级；已开对话仍需重开。
- 单测默认不打网：二进制名含 `.test` 且未设 `AGENT_AUTO_MODEL_RECOMMENDED_URL` 时跳过拉取（避免其它包测试误打 GitHub）。设了 URL 时即使是测试也会请求。`AGENT_AUTO_MODEL_SKIP_RECOMMENDED_CHECK=1` 或 `AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK=1` 同样跳过。
- 后台刷新子进程只设 `BACKGROUND_UPDATE_CHILD=1`，禁止再带 `SKIP_UPDATE_CHECK=1`，否则子进程什么都不干。

## 发版与双远端

- 公开协作 / Release / Homebrew 源：GitHub `x0c/agent-auto-model`
- 备份：Forgejo（见根/CLI `AGENTS.md` Remote 表）
- 本机收尾：`bash scripts/publish-release.sh vX.Y.Z`（不等 CI 排队）
- 配方仓库：复用 `x0c/homebrew-tap`，写入带版本回退防护

发版踩坑（必守）：

1. **新建配方必须先 `git add` 再看 staged diff**。对未跟踪文件跑 `git diff --quiet` 会误判「无需改动」，配方看起来更新成功、远端却没有文件。
2. **本机上传附件与 CI `release.yml` 可能抢同名资源导致 404**。附件失败不要中断配方更新；用户能否 `brew upgrade` 取决于配方指向的源码归档，不取决于预编译包是否齐。
3. **v* tag 推到 github 后立刻跑本机 `publish-release.sh`**；CI 只补其它平台包与配方兜底，不能当唯一升级通路。
4. **禁止 `git push --tags`**。只推 `main` 和本次 `vX.Y.Z`。本地若残留已在远端存在的旧 tag，`--tags` 会让整次推送被拒。

## 双机对齐

开发机 `~/Codes` 是这台 Mac 的同步镜像（路径仍写成 `/Users/geraltgraham/Codes`，用户是 `vibecoder`）。不要 ssh 成 root 去 `/root/Codes` 找项目。

产品目录改名或 GitHub / Forgejo 仓库改名后，**不要等同步自己完成**：

1. 开发机上若还是旧目录名：把它改成新名（同步常把改名当成删+建，旧目录会带着半成品脏文件留下来）。
2. `git remote` 仍可能指向旧仓库 URL，fetch 会报仓库不存在。按 `AGENTS.md` Remote 表改 `origin` 与 `github` 后再拉。
3. 快进到 `origin/main`，装上当前版本，清掉旧命令名 / 旧包装目录。
4. 两边都跑 `status`：包装路径是新目录、包装生效。同步面板里「还差几十个文件」可能是别的项目的 Git 对象，不能当成这个工具没对齐。
5. 开发机上 `go` 常常不在 PATH；用当前版本的 Linux 预编译包安装即可，不要卡在本机编译。
6. 开发机用户没有 GitHub SSH 时，`git fetch github` 会公钥失败；以 `origin`（Forgejo）是否跟上为准即可。
7. 核对 Homebrew 是否跟上：以 tap 仓库 `origin/main` 配方为准，不要只看 `raw.githubusercontent.com`（CDN 可能短暂落后）。

用户问「这台和开发机是不是最新、能不能跑」时按上面逐项核对，禁止只报本机 version。

## 独立性

本仓库保持独立产品边界，不并入其它会话托管 / Agent 启动链工具。
