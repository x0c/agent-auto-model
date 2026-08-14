// Package app 是 agent-auto-model 的命令入口。
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/x0c/agent-auto-model/internal/autoupdate"
	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/install"
	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/recommended"
	aamruntime "github.com/x0c/agent-auto-model/internal/runtime"
	"github.com/x0c/agent-auto-model/internal/runtime/codex"
	"github.com/x0c/agent-auto-model/internal/status"
	"github.com/x0c/agent-auto-model/internal/wrap"
)

// Version 由 -ldflags 注入。
var Version = "1.0.0"

// Run 执行 CLI。
func Run(args []string) int {
	if len(args) < 1 {
		return usage()
	}
	base := filepath.Base(args[0])
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".cmd")
	if aamruntime.IsWrapperName(base) {
		if err := execWrapper(paths.Home(), base, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return 0
	}
	if len(args) < 2 {
		return usage()
	}
	cmd := args[1]
	rest := args[2:]
	runPreflightUpdate(paths.Home(), cmd)
	switch cmd {
	case "version", "--version", "-V":
		fmt.Println(Version)
		return 0
	case "help", "--help", "-h":
		return usage()
	case "status":
		return cmdStatus(rest)
	case "install":
		return cmdInstall(rest)
	case "uninstall":
		return cmdUninstall(rest)
	case "exec":
		return cmdExec(rest)
	case "config":
		return cmdConfig(rest)
	case "update":
		return cmdUpdate(rest)
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%s\n", cmd)
		return usage()
	}
}

func execWrapper(home, invoked string, args []string) error {
	info, ok := aamruntime.RuntimeForWrapper(invoked)
	if !ok {
		return wrap.Exec(home, invoked, args)
	}
	switch info.Name {
	case config.RuntimeCodex:
		return codex.Exec(home, args)
	default:
		return wrap.Exec(home, invoked, args)
	}
}

func usage() int {
	fmt.Fprint(os.Stderr, `agent-auto-model — auto-switch agent CLI models by Mode

用法：
  agent-auto-model status [--runtime cursor|codex|all] [--json]
  agent-auto-model install [--runtime ...] [--dry-run] [--json]
  agent-auto-model uninstall [--runtime ...] [--dry-run] [--json]
  agent-auto-model update [--force] [--quiet] [--json]
  agent-auto-model config show [--json]
  agent-auto-model config set <mode|runtime.mode> <model-id> [--json]
  agent-auto-model config set-many plan=... codex.plan=... [--json]
  agent-auto-model config enable|disable [--runtime ...] [--json]
  agent-auto-model config set-strict true|false [--json]
  agent-auto-model config set-auto-update true|false [--json]
  agent-auto-model config set-update-interval <hours> [--json]
  agent-auto-model config set-models-source recommended|local [--json]
  agent-auto-model config refresh-recommended [--json]
  agent-auto-model config reset [--json]
  agent-auto-model exec [--invoked-as NAME] -- [cli 参数...]
  agent-auto-model version

mode：plan / default / search / debug（Cursor；CLI --mode ask 对应 search）
runtime.mode：codex.plan / codex.default；Cursor 可用 cursor.plan 或省略前缀。

环境变量：
  AGENT_AUTO_MODEL=0           总开关关闭
  AGENT_AUTO_MODEL_CONFIG      预加载读取的配置路径
  AGENT_AUTO_MODEL_LOCK=1      本会话禁止自动切换（显式 --model 时由包装自动设置）
  AGENT_AUTO_MODEL_RECOMMENDED_URL  覆盖推荐配置拉取地址
`)
	return 2
}

func cmdStatus(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	filter, err := runtimeNamesFromArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	p := status.Collect(paths.Home(), filter)
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": p, "meta": map[string]any{"version": Version}})
	}
	active := "否"
	if p.Active {
		active = "是"
	}
	fmt.Printf("agent-auto-model：%s（active=%s）\n", map[bool]string{true: "启用中", false: "未启用"}[p.Active], active)
	fmt.Printf("  配置：%s\n", p.UserConfig)
	fmt.Printf("  预加载：%s（存在=%v）\n", p.Register, p.RegisterOK)
	fmt.Printf("  包装目录：%s\n", p.WrapperBin)
	fmt.Printf("  严格模式：%v\n", p.Strict)
	fmt.Printf("  模型映射来源：%s（%s）\n", p.ModelsSourceTag, p.ModelsSource)
	fmt.Printf("  推荐配置：来源=%s 上次检查=%s\n",
		emptyDash(p.Recommended.SourceTag), emptyDash(p.Recommended.LastCheckedAt))
	if p.Recommended.LastError != "" {
		fmt.Printf("  推荐配置错误：%s\n", p.Recommended.LastError)
	}
	for _, rt := range p.Runtimes {
		fmt.Printf("  [%s] enabled=%v wrapper=%v\n", rt.Name, rt.Enabled, rt.WrapperEffective)
		if rt.RealBinary != "" {
			fmt.Printf("    官方：%s\n", rt.RealBinary)
		}
		for _, mode := range config.ValidModesFor(rt.Name) {
			fmt.Printf("    %s → %s\n", mode, rt.Models[mode])
		}
		if rt.Anchors != nil {
			parts := make([]string, 0, len(rt.Anchors.Anchors))
			for k, v := range rt.Anchors.Anchors {
				mark := "MISS"
				if v {
					mark = "OK"
				}
				parts = append(parts, k+"="+mark)
			}
			fmt.Printf("    锚点：%s\n", strings.Join(parts, ", "))
		}
		if len(rt.RecentDecisions) > 0 {
			fmt.Println("    最近决策：")
			for _, d := range rt.RecentDecisions {
				fmt.Printf("      - ev=%s mode=%s expected=%s actual=%s\n",
					emptyDash(d.Ev), emptyDash(d.Mode), emptyDash(d.Expected), emptyDash(d.Actual))
			}
		}
		if rt.Hint != "" {
			fmt.Printf("    提示：%s\n", rt.Hint)
		}
	}
	fmt.Printf("  自更新：enabled=%v channel=%s interval=%dh last=%s installed=%s\n",
		p.AutoUpdate.Enabled,
		emptyDash(p.AutoUpdate.Channel),
		p.AutoUpdate.CheckIntervalHours,
		emptyDash(p.AutoUpdate.LastCheckedAt),
		emptyDash(p.AutoUpdate.LastInstalled))
	if p.AutoUpdate.LastError != "" {
		fmt.Printf("  自更新错误：%s\n", p.AutoUpdate.LastError)
	}
	if p.Hint != "" {
		fmt.Printf("  提示：%s\n", p.Hint)
	}
	return 0
}

func cmdInstall(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	dry := hasFlag(args, "--dry-run")
	names, err := runtimeNamesFromArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	res, err := install.Install(paths.Home(), self, dry, names)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": res, "meta": map[string]any{"version": Version}})
	}
	fmt.Printf("已安装（enabled=%v）\n", res.Enabled)
	fmt.Printf("  配置：%s\n", res.UserConfig)
	fmt.Printf("  预加载：%s\n", res.Register)
	fmt.Printf("  包装目录：%s\n", res.WrapperBin)
	if res.PathHint != "" {
		fmt.Printf("  请确保 PATH 优先：%s\n", res.PathHint)
		fmt.Println("  （已尝试写入 shell rc / 用户 PATH；新开终端后生效）")
	}
	return 0
}

func cmdUninstall(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	dry := hasFlag(args, "--dry-run")
	names, err := runtimeNamesFromArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	res, err := install.Uninstall(paths.Home(), dry, names)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": res, "meta": map[string]any{"version": Version}})
	}
	fmt.Printf("已关闭自动切换（enabled=%v），包装已移除\n", res.Enabled)
	return 0
}

func cmdExec(args []string) int {
	invoked := "agent"
	rest := args
	for len(rest) > 0 {
		if rest[0] == "--invoked-as" && len(rest) >= 2 {
			invoked = rest[1]
			rest = rest[2:]
			continue
		}
		if rest[0] == "--" {
			rest = rest[1:]
			break
		}
		break
	}
	if self, err := os.Executable(); err == nil {
		autoupdate.KickoffBackgroundCheck(paths.Home(), self)
	}
	if err := execWrapper(paths.Home(), invoked, rest); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func cmdConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "缺少 config 子命令")
		return usage()
	}
	sub := args[0]
	rest := args[1:]
	wantJSON := hasFlag(rest, "--json") || !stdoutIsTTY()
	home := paths.Home()
	restartHint := "已写入配置。已打开的 Agent 会话需重启后才会使用新映射。"

	switch sub {
	case "show":
		cfg := config.LoadEffective(home)
		rec := recommended.Status(home)
		if wantJSON {
			return writeJSON(map[string]any{
				"ok":   true,
				"data": cfg,
				"meta": map[string]any{
					"version":           Version,
					"models_source_tag": config.ModelsSourceTag(cfg.ModelsSource),
					"recommended":       rec,
				},
			})
		}
		printConfig(cfg, rec)
		return 0
	case "set":
		rest = stripFlag(rest, "--json")
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "用法：config set <mode|runtime.mode> <model-id>")
			return 2
		}
		rt, mode, err := config.ParseTarget(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		if !strings.Contains(rest[0], ".") {
			fmt.Fprintln(os.Stderr, "提示：未写 runtime 前缀时视为 cursor."+mode+"；Codex 请用 config set codex.plan ...")
		}
		before := config.Load(home)
		cfg, err := config.SetRuntimeModel(home, rt, mode, rest[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		hint := restartHint
		switched := before.ModelsSource == config.ModelsSourceRecommended && cfg.ModelsSource == config.ModelsSourceLocal
		if switched {
			hint = sourceSwitchedHint()
		}
		return configMutated(wantJSON, cfg, hint, switched)
	case "set-many":
		rest = stripFlag(rest, "--json")
		pairs := map[string]string{}
		for _, item := range rest {
			k, v, ok := strings.Cut(item, "=")
			if !ok || k == "" || v == "" {
				fmt.Fprintf(os.Stderr, "非法参数 %q，期望 mode=model-id 或 runtime.mode=model-id\n", item)
				return 2
			}
			pairs[k] = v
		}
		if len(pairs) == 0 {
			fmt.Fprintln(os.Stderr, "用法：config set-many plan=... codex.plan=...")
			return 2
		}
		before := config.Load(home)
		cfg, err := config.SetMany(home, pairs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		hint := restartHint
		switched := before.ModelsSource == config.ModelsSourceRecommended && cfg.ModelsSource == config.ModelsSourceLocal
		if switched {
			hint = sourceSwitchedHint()
		}
		return configMutated(wantJSON, cfg, hint, switched)
	case "enable", "disable":
		enabled := sub == "enable"
		filter, err := runtimeNamesFromArgs(rest)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 2
		}
		var cfg config.Config
		if len(filter) == 0 {
			cfg, err = config.SetEnabled(home, enabled)
		} else {
			cfg = config.Load(home)
			for _, name := range filter {
				cfg, err = config.SetRuntimeEnabled(home, name, enabled)
				if err != nil {
					break
				}
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint, false)
	case "set-strict":
		rest = stripFlag(rest, "--json")
		rest = stripRuntimeFlags(rest)
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "用法：config set-strict true|false")
			return 2
		}
		strict, err := strconv.ParseBool(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "set-strict 需要 true 或 false")
			return 2
		}
		cfg, err := config.SetStrict(home, strict)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint, false)
	case "set-auto-update":
		rest = stripFlag(rest, "--json")
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "用法：config set-auto-update true|false")
			return 2
		}
		enabled, err := strconv.ParseBool(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "set-auto-update 需要 true 或 false")
			return 2
		}
		cfg, err := config.SetAutoUpdateEnabled(home, enabled)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, "已写入配置。静默自更新开关将在下一次命令执行时生效。", false)
	case "set-update-interval":
		rest = stripFlag(rest, "--json")
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "用法：config set-update-interval <hours>")
			return 2
		}
		hours, err := strconv.Atoi(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "set-update-interval 需要正整数小时数")
			return 2
		}
		cfg, err := config.SetAutoUpdateInterval(home, hours)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, "已写入配置。新的静默自更新检查间隔将在下一次命令执行时生效。", false)
	case "set-models-source":
		rest = stripFlag(rest, "--json")
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "用法：config set-models-source recommended|local")
			return 2
		}
		stored := config.Load(home)
		src, err := config.ParseModelsSource(rest[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 2
		}
		hint := restartHint
		if src == config.ModelsSourceRecommended && stored.ModelsSource == config.ModelsSourceLocal && !config.MatchesRecommended(home, stored) {
			hint = "已切换为推荐配置。这会覆盖你改过的模型映射。已打开的 Agent 会话需重启后才会使用新映射。"
		}
		cfg, err := config.SetModelsSource(home, src)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, hint, false)
	case "refresh-recommended":
		rest = stripFlag(rest, "--json")
		err := recommended.MaybeRefresh(home, true)
		cfg := config.LoadEffective(home)
		if cfg.ModelsSource == config.ModelsSourceRecommended {
			_ = config.SyncRuntime(home, cfg)
		}
		rec := recommended.Status(home)
		if wantJSON {
			meta := map[string]any{"version": Version, "recommended": rec}
			if err != nil {
				meta["error"] = err.Error()
			}
			return writeJSON(map[string]any{"ok": err == nil, "data": cfg, "meta": meta})
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "刷新推荐配置失败：%v\n", err)
			printConfig(cfg, rec)
			return 1
		}
		fmt.Println("已刷新推荐配置。来源为推荐配置时，已打开的会话需重启后才会使用新映射。")
		printConfig(cfg, rec)
		return 0
	case "reset":
		cfg, err := config.Reset(home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint, false)
	default:
		fmt.Fprintf(os.Stderr, "未知 config 子命令：%s\n", sub)
		return usage()
	}
}

func configMutated(wantJSON bool, cfg config.Config, hint string, sourceSwitched bool) int {
	home := paths.Home()
	cfg = config.ApplyRecommended(home, cfg)
	rec := recommended.Status(home)
	if wantJSON {
		return writeJSON(map[string]any{
			"ok":   true,
			"data": cfg,
			"meta": map[string]any{
				"version":                Version,
				"hint":                   hint,
				"models_source_switched": sourceSwitched,
				"models_source_tag":      config.ModelsSourceTag(cfg.ModelsSource),
				"recommended":            rec,
			},
		})
	}
	fmt.Println(hint)
	printConfig(cfg, rec)
	return 0
}

func sourceSwitchedHint() string {
	return "已改为本地自定义，不再跟随仓库推荐配置。已打开的 Agent 会话需重启后才会使用新映射。切回推荐：agent-auto-model config set-models-source recommended"
}

func printConfig(cfg config.Config, rec recommended.RuntimeStatus) {
	fmt.Printf("enabled=%v strict=%v version=%d\n", cfg.Enabled, cfg.Strict, cfg.Version)
	fmt.Printf("  模型映射来源 → %s（%s）\n", config.ModelsSourceTag(cfg.ModelsSource), cfg.ModelsSource)
	fmt.Printf("  推荐配置来源 → %s 上次检查=%s\n", emptyDash(rec.SourceTag), emptyDash(rec.LastCheckedAt))
	if rec.LastError != "" {
		fmt.Printf("  推荐配置错误 → %s\n", rec.LastError)
	}
	for _, name := range config.ValidRuntimes {
		rt := cfg.Runtimes[name]
		fmt.Printf("  [%s] enabled=%v\n", name, rt.Enabled)
		for _, mode := range config.ValidModesFor(name) {
			fmt.Printf("    %s → %s\n", mode, rt.Models[mode])
		}
	}
	fmt.Printf("  auto_update.enabled → %v\n", cfg.AutoUpdate.Enabled)
	fmt.Printf("  auto_update.interval_hours → %d\n", cfg.AutoUpdate.CheckIntervalHours)
	fmt.Printf("  auto_update.channel → %s\n", cfg.AutoUpdate.Channel)
}

func cmdUpdate(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	force := hasFlag(args, "--force")
	quiet := hasFlag(args, "--quiet")
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	err = autoupdate.MaybeCheckAndUpdate(paths.Home(), Version, self, force)
	p := status.Collect(paths.Home(), nil)
	if wantJSON {
		meta := map[string]any{"version": Version}
		if err != nil {
			meta["error"] = err.Error()
		}
		return writeJSON(map[string]any{"ok": err == nil, "data": p, "meta": meta})
	}
	if quiet {
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "静默自更新失败：%v\n", err)
		return 1
	}
	fmt.Printf("静默自更新检查完成。当前已安装：%s\n", emptyDash(p.AutoUpdate.LastInstalled))
	return 0
}

func runPreflightUpdate(home, cmd string) {
	if os.Getenv("AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK") == "1" {
		return
	}
	switch cmd {
	case "install", "uninstall", "version", "--version", "-V", "help", "--help", "-h", "update", "exec":
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	switch cmd {
	case "status", "config":
		autoupdate.KickoffBackgroundCheck(home, self)
		return
	}
	_ = autoupdate.MaybeCheckAndUpdate(home, Version, self, false)
}

func runtimeNamesFromArgs(args []string) ([]string, error) {
	out := parseRuntimeFilter(args)
	for _, name := range out {
		if !config.IsValidRuntime(name) {
			return nil, fmt.Errorf("非法 runtime %q（允许：%s）", name, strings.Join(config.ValidRuntimes, ", "))
		}
	}
	return out, nil
}

func parseRuntimeFilter(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		var val string
		if a == "--runtime" && i+1 < len(args) {
			val = args[i+1]
			i++
		} else if strings.HasPrefix(a, "--runtime=") {
			val = strings.TrimPrefix(a, "--runtime=")
		} else {
			continue
		}
		for _, part := range strings.Split(val, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part == "" || part == "all" {
				return nil
			}
			out = append(out, part)
		}
	}
	return out
}

func stripRuntimeFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--runtime" {
			i++
			continue
		}
		if strings.HasPrefix(a, "--runtime=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func stripFlag(args []string, name string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == name {
			continue
		}
		out = append(out, a)
	}
	return out
}

func stdoutIsTTY() bool {
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

func writeJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
