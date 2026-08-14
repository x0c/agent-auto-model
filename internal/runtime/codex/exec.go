package codex

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/x0c/agent-auto-model/internal/config"
)

// Exec 拦截交互式 TUI：经 app-server 代理按 Mode 改写模型；其它子命令原样转调。
func Exec(home string, args []string) error {
	real, err := FindReal(home)
	if err != nil {
		return err
	}
	if config.GloballyDisabled() {
		return runReal(real, args)
	}
	cfg := config.LoadEffective(home)
	if !config.RuntimeEnabled(cfg, config.RuntimeCodex) {
		return runReal(real, args)
	}
	if isPassthrough(args) {
		return runReal(real, args)
	}
	locked := argvHasExplicitModel(args)
	if err := runProxiedTUI(home, real, args, locked); err != nil {
		fmt.Fprintf(os.Stderr, "[agent-auto-model] Codex 代理失败，回退官方入口：%v\n", err)
		return runReal(real, args)
	}
	return nil
}

func runReal(real string, args []string) error {
	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return fmt.Errorf("启动 Codex 失败：%w", err)
	}
	return nil
}
