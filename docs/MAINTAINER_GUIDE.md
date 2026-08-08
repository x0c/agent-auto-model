# 维护指南

## Mode→模型挂钩

预加载脚本：`internal/assets/register.mjs`（经 `go:embed` 打进二进制，install/exec 时落到 `~/.local/share/cursor-mode-model/assets/`）。

锚点（Agent `2026.08.04-aaa8809` 核实）：

- `setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){`
- `setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){`
- `setMetadata(e,t){this.metadataStore.set(e,t)}`

升级 Cursor Agent 后先跑 `cursor-mode-model status`。锚点未命中则自动切换失效，但 Agent 仍可正常用（故障开放）。

## 包装与 PATH

`install` 在 `~/.local/share/cursor-mode-model/bin/` 写入 `agent` / `cursor-agent` 壳脚本，并尽量把该目录 prepend 进 `~/.zshrc` / `~/.bashrc`。官方入口仍在 `~/.local/share/cursor-agent/versions/<ver>/cursor-agent`；包装通过扫 versions 目录找最新一份，不依赖会被 `agent update` 改写的 `~/.local/bin` 符号链接。

## 与 pickup 的边界

本工具独立。即使用户同时装了 pickup 的命令拦截，只要包装目录在 PATH 更靠前，或 pickup 最终 `execvp("agent")` 走到本包装，Mode→模型就会生效。禁止把本逻辑并回 pickup 仓库。
