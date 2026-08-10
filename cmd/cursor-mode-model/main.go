package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/config"
	"github.com/x0c/cursor-mode-model/internal/install"
	"github.com/x0c/cursor-mode-model/internal/paths"
	"github.com/x0c/cursor-mode-model/internal/status"
	"github.com/x0c/cursor-mode-model/internal/wrap"
)

// 由 -ldflags 注入。
var version = "0.2.0"

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 1 {
		return usage()
	}
	base := filepath.Base(args[0])
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".cmd")
	// 被包装脚本以 agent / cursor-agent 名义调用时，直接 exec。
	if base == "agent" || base == "cursor-agent" {
		if err := wrap.Exec(paths.Home(), base, args[1:]); err != nil {
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
	switch cmd {
	case "version", "--version", "-V":
		fmt.Println(version)
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
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%s\n", cmd)
		return usage()
	}
}

func usage() int {
	fmt.Fprint(os.Stderr, `cursor-mode-model — auto-switch Cursor Agent CLI models by Mode

用法：
  cursor-mode-model status [--json]
  cursor-mode-model install [--dry-run] [--json]
  cursor-mode-model uninstall [--dry-run] [--json]
  cursor-mode-model config show [--json]
  cursor-mode-model config set <mode> <model-id> [--json]
  cursor-mode-model config set-many plan=... default=... [--json]
  cursor-mode-model config enable|disable [--json]
  cursor-mode-model config set-strict true|false [--json]
  cursor-mode-model config reset [--json]
  cursor-mode-model exec [--invoked-as NAME] -- [agent 参数...]
  cursor-mode-model version

mode 合法值：plan / default / search / debug（CLI --mode ask 对应 search）

环境变量：
  CURSOR_MODE_MODEL=0          总开关关闭
  CURSOR_MODE_MODEL_CONFIG     预加载读取的配置路径
  CURSOR_MODE_MODEL_LOCK=1     本会话禁止自动切换（显式 --model 时由包装自动设置）
`)
	return 2
}

func cmdStatus(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	p := status.Collect(paths.Home())
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": p, "meta": map[string]any{"version": version}})
	}
	active := "否"
	if p.Active {
		active = "是"
	}
	fmt.Printf("Cursor Mode→模型：%s（active=%s）\n", map[bool]string{true: "启用中", false: "未启用"}[p.Active], active)
	fmt.Printf("  配置：%s\n", p.UserConfig)
	fmt.Printf("  预加载：%s（存在=%v）\n", p.Register, p.RegisterOK)
	fmt.Printf("  包装目录：%s\n", p.WrapperBin)
	fmt.Printf("  PATH 上的 agent：%s\n", emptyDash(p.PathAgent))
	fmt.Printf("  官方 agent：%s\n", emptyDash(p.RealAgent))
	fmt.Printf("  严格模式：%v\n", p.Strict)
	fmt.Printf("  Plan → %s\n", p.Models["plan"])
	fmt.Printf("  其它 → %s\n", p.Models["default"])
	parts := make([]string, 0, len(p.Anchors.Anchors))
	for k, v := range p.Anchors.Anchors {
		mark := "MISS"
		if v {
			mark = "OK"
		}
		parts = append(parts, k+"="+mark)
	}
	fmt.Printf("  锚点：%s\n", strings.Join(parts, ", "))
	if len(p.RecentDecisions) > 0 {
		fmt.Println("  最近决策：")
		for _, d := range p.RecentDecisions {
			fmt.Printf("    - ev=%s mode=%s expected=%s actual=%s\n",
				emptyDash(d.Ev), emptyDash(d.Mode), emptyDash(d.Expected), emptyDash(d.Actual))
		}
	}
	if p.Hint != "" {
		fmt.Printf("  提示：%s\n", p.Hint)
	}
	return 0
}

func cmdInstall(args []string) int {
	wantJSON := hasFlag(args, "--json") || !stdoutIsTTY()
	dry := hasFlag(args, "--dry-run")
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	res, err := install.Install(paths.Home(), self, dry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": res, "meta": map[string]any{"version": version}})
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
	res, err := install.Uninstall(paths.Home(), dry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": res, "meta": map[string]any{"version": version}})
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
	if err := wrap.Exec(paths.Home(), invoked, rest); err != nil {
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
		cfg := config.Load(home)
		if wantJSON {
			return writeJSON(map[string]any{"ok": true, "data": cfg, "meta": map[string]any{"version": version}})
		}
		printConfig(cfg)
		return 0
	case "set":
		rest = stripFlag(rest, "--json")
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "用法：config set <mode> <model-id>")
			return 2
		}
		cfg, err := config.SetModel(home, rest[0], rest[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		if wantJSON {
			return writeJSON(map[string]any{"ok": true, "data": cfg, "meta": map[string]any{"version": version, "hint": restartHint}})
		}
		fmt.Println(restartHint)
		printConfig(cfg)
		return 0
	case "set-many":
		rest = stripFlag(rest, "--json")
		pairs := map[string]string{}
		for _, item := range rest {
			k, v, ok := strings.Cut(item, "=")
			if !ok || k == "" || v == "" {
				fmt.Fprintf(os.Stderr, "非法参数 %q，期望 mode=model-id\n", item)
				return 2
			}
			pairs[k] = v
		}
		if len(pairs) == 0 {
			fmt.Fprintln(os.Stderr, "用法：config set-many plan=... default=...")
			return 2
		}
		cfg, err := config.SetMany(home, pairs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		if wantJSON {
			return writeJSON(map[string]any{"ok": true, "data": cfg, "meta": map[string]any{"version": version, "hint": restartHint}})
		}
		fmt.Println(restartHint)
		printConfig(cfg)
		return 0
	case "enable":
		cfg, err := config.SetEnabled(home, true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint)
	case "disable":
		cfg, err := config.SetEnabled(home, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint)
	case "set-strict":
		rest = stripFlag(rest, "--json")
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
		return configMutated(wantJSON, cfg, restartHint)
	case "reset":
		cfg, err := config.Reset(home)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return configMutated(wantJSON, cfg, restartHint)
	default:
		fmt.Fprintf(os.Stderr, "未知 config 子命令：%s\n", sub)
		return usage()
	}
}

func configMutated(wantJSON bool, cfg config.Config, hint string) int {
	if wantJSON {
		return writeJSON(map[string]any{"ok": true, "data": cfg, "meta": map[string]any{"version": version, "hint": hint}})
	}
	fmt.Println(hint)
	printConfig(cfg)
	return 0
}

func printConfig(cfg config.Config) {
	fmt.Printf("enabled=%v strict=%v\n", cfg.Enabled, cfg.Strict)
	for _, mode := range config.ValidModes {
		fmt.Printf("  %s → %s\n", mode, cfg.Models[mode])
	}
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
