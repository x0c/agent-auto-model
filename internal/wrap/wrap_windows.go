//go:build windows

package wrap

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/x0c/cursor-mode-model/internal/agentbin"
)

// Exec 解析官方 agent，注入环境后运行（Windows 无进程替换）。
func Exec(home string, argv0 string, args []string) error {
	real, err := agentbin.Find(home)
	if err != nil {
		return err
	}
	extra, err := PrepareEnv(home, args)
	if err != nil {
		return err
	}
	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	env := os.Environ()
	for k, v := range extra {
		env = upsertEnv(env, k, v)
	}
	cmd.Env = env
	if argv0 != "" {
		cmd.Args[0] = argv0
	}
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return fmt.Errorf("启动 Cursor Agent 失败：%w", err)
	}
	return nil
}
