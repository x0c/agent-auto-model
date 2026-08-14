//go:build unix

package wrap

import (
	"fmt"
	"os"
	"syscall"

	"github.com/x0c/agent-auto-model/internal/agentbin"
)

// Exec 解析官方 agent，注入环境后替换当前进程。
func Exec(home string, argv0 string, args []string) error {
	real, err := agentbin.Find(home)
	if err != nil {
		return err
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
	if err := syscall.Exec(real, argv, env); err != nil {
		return fmt.Errorf("启动 Cursor Agent 失败：%w", err)
	}
	return nil
}
