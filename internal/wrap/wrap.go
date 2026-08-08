// Package wrap 在启动官方 Agent 前注入 Node 预加载环境。
package wrap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/agentbin"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/assets"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/config"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

// Exec 解析官方 agent，注入环境后替换当前进程。
func Exec(home string, argv0 string, args []string) error {
	real, err := agentbin.Find(home)
	if err != nil {
		return fmt.Errorf("未找到 Cursor Agent 安装：%w", err)
	}
	extra, err := PrepareEnv(home, args)
	if err != nil {
		return err
	}
	env := os.Environ()
	for k, v := range extra {
		env = upsertEnv(env, k, v)
	}
	if argv0 == "" {
		argv0 = "agent"
	}
	argv := append([]string{argv0}, args...)
	return syscall.Exec(real, argv, env)
}

// PrepareEnv 返回应合并进环境的键值；关闭时返回空 map。
func PrepareEnv(home string, args []string) (map[string]string, error) {
	out := map[string]string{}
	if config.GloballyDisabled() {
		return out, nil
	}
	cfg := config.Load(home)
	if !cfg.Enabled {
		return out, nil
	}
	register, err := assets.Ensure(home)
	if err != nil {
		return nil, err
	}
	if err := config.SyncRuntime(home, cfg); err != nil {
		return nil, err
	}
	out[paths.EnvConfig] = paths.RuntimeConfigFile(home)
	out["NODE_OPTIONS"] = appendFlag(os.Getenv("NODE_OPTIONS"), "--import="+register)
	if argvHasExplicitModel(args) {
		out[paths.EnvLock] = "1"
	}
	return out, nil
}

func argvHasExplicitModel(args []string) bool {
	for _, a := range args {
		if a == "--model" || strings.HasPrefix(a, "--model=") {
			return true
		}
	}
	return false
}

func appendFlag(existing, flag string) string {
	parts := strings.Fields(existing)
	for _, p := range parts {
		if p == flag {
			return strings.Join(parts, " ")
		}
		if strings.HasPrefix(p, "--import=") && strings.Contains(p, "cursor-mode-model") {
			return strings.Join(parts, " ")
		}
	}
	parts = append(parts, flag)
	return strings.Join(parts, " ")
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// LookPath 供诊断：当前 PATH 上的可执行文件。
func LookPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}
