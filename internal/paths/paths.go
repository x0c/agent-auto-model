// Package paths 定义本工具的配置、资产与包装目录。
package paths

import (
	"os"
	"path/filepath"
)

const (
	AppName    = "agent-auto-model"
	LegacyName = "cursor-mode-model"

	EnvDisable = "AGENT_AUTO_MODEL"
	EnvConfig  = "AGENT_AUTO_MODEL_CONFIG"
	EnvLock    = "AGENT_AUTO_MODEL_LOCK"
	EnvHome    = "AGENT_AUTO_MODEL_HOME"

	LegacyEnvDisable = "CURSOR_MODE_MODEL"
	LegacyEnvConfig  = "CURSOR_MODE_MODEL_CONFIG"
	LegacyEnvLock    = "CURSOR_MODE_MODEL_LOCK"
	LegacyEnvHome    = "CURSOR_MODE_MODEL_HOME"
)

// Home 返回用户主目录，可被测试注入。
func Home() string {
	if h := firstNonEmpty(os.Getenv(EnvHome), os.Getenv(LegacyEnvHome)); h != "" {
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
		return filepath.Join(xdg, AppName)
	}
	return filepath.Join(home, ".config", AppName)
}

func LegacyConfigDir(home string) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, LegacyName)
	}
	return filepath.Join(home, ".config", LegacyName)
}

func DataDir(home string) string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, AppName)
	}
	return filepath.Join(home, ".local", "share", AppName)
}

func LegacyDataDir(home string) string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, LegacyName)
	}
	return filepath.Join(home, ".local", "share", LegacyName)
}

func UserConfigFile(home string) string {
	return filepath.Join(ConfigDir(home), "config.json")
}

func LegacyUserConfigFile(home string) string {
	return filepath.Join(LegacyConfigDir(home), "config.json")
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

func DecisionsLog(home string) string {
	return filepath.Join(AssetsDir(home), "decisions.log")
}

func CodexDecisionsLog(home string) string {
	return filepath.Join(AssetsDir(home), "codex-decisions.log")
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

func EnvOrLegacy(primary, legacy string) string {
	return firstNonEmpty(os.Getenv(primary), os.Getenv(legacy))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
