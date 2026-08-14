package codex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/x0c/agent-auto-model/internal/agentbin"
	"github.com/x0c/agent-auto-model/internal/paths"
)

func wrapperNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"codex.cmd", "codex.exe", "codex.bat", "codex"}
	}
	return []string{"codex"}
}

// FindReal 定位官方 codex，跳过本工具包装层。
func FindReal(home string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("AGENT_AUTO_MODEL_CODEX")); override != "" {
		if st, err := os.Stat(override); err == nil && !st.IsDir() {
			return override, nil
		}
	}
	wrapperDir := paths.WrapperBinDir(home)
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}
		if absDir == wrapperDir {
			continue
		}
		for _, name := range wrapperNames() {
			p := filepath.Join(dir, name)
			st, err := os.Stat(p)
			if err != nil || st.IsDir() {
				continue
			}
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			if agentbin.IsOurWrapper(home, abs) {
				continue
			}
			if runtime.GOOS != "windows" && st.Mode()&0o111 == 0 {
				continue
			}
			return abs, nil
		}
	}
	if p, err := exec.LookPath("codex"); err == nil && !agentbin.IsOurWrapper(home, p) {
		return p, nil
	}
	return "", fmt.Errorf("未找到 Codex CLI。请先安装：npm install -g @openai/codex")
}

func argvHasExplicitModel(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--model" || a == "-m" {
			return true
		}
		if strings.HasPrefix(a, "--model=") || strings.HasPrefix(a, "-m=") {
			return true
		}
	}
	return false
}

func argvHasRemote(args []string) bool {
	for _, a := range args {
		if a == "--remote" || strings.HasPrefix(a, "--remote=") {
			return true
		}
	}
	return false
}

var passthroughSubcommands = map[string]bool{
	"exec": true, "e": true, "review": true, "login": true, "logout": true,
	"mcp": true, "plugin": true, "mcp-server": true, "app-server": true,
	"remote-control": true, "completion": true, "update": true, "doctor": true,
	"sandbox": true, "debug": true, "apply": true, "a": true,
	"archive": true, "delete": true, "unarchive": true, "fork": true,
	"cloud": true, "exec-server": true, "features": true, "help": true,
}

func isPassthrough(args []string) bool {
	if argvHasRemote(args) {
		return true
	}
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "-h" || a == "--help" || a == "-V" || a == "--version" {
			return true
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return passthroughSubcommands[a]
	}
	return false
}
