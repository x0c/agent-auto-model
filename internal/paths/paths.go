// Package paths 定义本工具的配置、资产与包装目录。
package paths

import (
	"os"
	"path/filepath"
)

const (
	EnvDisable = "CURSOR_MODE_MODEL"
	EnvConfig  = "CURSOR_MODE_MODEL_CONFIG"
	EnvLock    = "CURSOR_MODE_MODEL_LOCK"
)

// Home 返回用户主目录，可被测试注入。
func Home() string {
	if h := os.Getenv("CURSOR_MODE_MODEL_HOME"); h != "" {
		return h
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func ConfigDir(home string) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cursor-mode-model")
	}
	return filepath.Join(home, ".config", "cursor-mode-model")
}

func DataDir(home string) string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "cursor-mode-model")
	}
	return filepath.Join(home, ".local", "share", "cursor-mode-model")
}

func UserConfigFile(home string) string {
	return filepath.Join(ConfigDir(home), "config.json")
}

func AssetsDir(home string) string {
	return filepath.Join(DataDir(home), "assets")
}

func RegisterFile(home string) string {
	return filepath.Join(AssetsDir(home), "register.mjs")
}

func RuntimeConfigFile(home string) string {
	return filepath.Join(AssetsDir(home), "config.json")
}

func WrapperBinDir(home string) string {
	return filepath.Join(DataDir(home), "bin")
}

func StateFile(home string) string {
	return filepath.Join(DataDir(home), "state.json")
}

func AutoUpdateStateFile(home string) string {
	return filepath.Join(DataDir(home), "autoupdate.json")
}

func AutoUpdateLockFile(home string) string {
	return filepath.Join(DataDir(home), "autoupdate.lock")
}

// CursorAgentVersionsDirs 返回可能的 Cursor Agent versions 目录（按优先级）。
func CursorAgentVersionsDirs(home string) []string {
	out := []string{
		filepath.Join(home, ".local", "share", "cursor-agent", "versions"),
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		out = append(out, filepath.Join(local, "cursor-agent", "versions"))
	}
	return out
}

// CursorAgentVersionsDir 兼容旧调用：返回第一个候选路径。
func CursorAgentVersionsDir(home string) string {
	dirs := CursorAgentVersionsDirs(home)
	if len(dirs) == 0 {
		return ""
	}
	return dirs[0]
}

func LocalBinDir(home string) string {
	return filepath.Join(home, ".local", "bin")
}
