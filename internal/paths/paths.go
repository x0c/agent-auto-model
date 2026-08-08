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

func CursorAgentVersionsDir(home string) string {
	return filepath.Join(home, ".local", "share", "cursor-agent", "versions")
}

func LocalBinDir(home string) string {
	return filepath.Join(home, ".local", "bin")
}
