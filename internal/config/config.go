// Package config 读写 Mode→模型映射配置。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/paths"
)

// DefaultModels 默认映射：Plan→Opus 5，其它→当前最新 Grok high（通配符运行时解析）。
var DefaultModels = map[string]string{
	"plan":    "claude-opus-5-thinking-high",
	"default": "cursor-grok-*-high",
	"search":  "cursor-grok-*-high",
	"debug":   "cursor-grok-*-high",
}

const (
	DefaultAutoUpdateChannel            = "github_release"
	DefaultAutoUpdateCheckIntervalHours = 24
)

// ValidModes 可配置的 Mode 键（CLI --mode ask 对应内部 search）。
var ValidModes = []string{"plan", "default", "search", "debug"}

// Config 用户可改配置。
type Config struct {
	Version    int               `json:"version"`
	Enabled    bool              `json:"enabled"`
	Strict     bool              `json:"strict"`
	Models     map[string]string `json:"models"`
	AutoUpdate AutoUpdateConfig  `json:"auto_update"`
}

// AutoUpdateConfig 控制静默自更新。
type AutoUpdateConfig struct {
	Enabled            bool   `json:"enabled"`
	CheckIntervalHours int    `json:"check_interval_hours"`
	Channel            string `json:"channel"`
}

// Default 返回默认配置副本。
func Default() Config {
	models := make(map[string]string, len(DefaultModels))
	for k, v := range DefaultModels {
		models[k] = v
	}
	return Config{
		Version: 1,
		Enabled: true,
		Strict:  false,
		Models:  models,
		AutoUpdate: AutoUpdateConfig{
			Enabled:            true,
			CheckIntervalHours: DefaultAutoUpdateCheckIntervalHours,
			Channel:            DefaultAutoUpdateChannel,
		},
	}
}

// IsValidMode 判断 mode 键是否合法。
func IsValidMode(mode string) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}

// NormalizeModeAlias 把 ask→search 等别名归一。
func NormalizeModeAlias(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "ask":
		return "search"
	case "agent":
		return "default"
	default:
		return strings.TrimSpace(mode)
	}
}

// Load 读取用户配置；不存在或损坏时返回默认。
// enabled 字段缺省时按启用处理（避免 JSON 布尔零值把缺省当成关闭）。
func Load(home string) Config {
	path := paths.UserConfigFile(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	var raw struct {
		Version    int               `json:"version"`
		Enabled    *bool             `json:"enabled"`
		Strict     *bool             `json:"strict"`
		Models     map[string]string `json:"models"`
		AutoUpdate *struct {
			Enabled            *bool  `json:"enabled"`
			CheckIntervalHours *int   `json:"check_interval_hours"`
			Channel            string `json:"channel"`
		} `json:"auto_update"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Default()
	}
	out := Default()
	if raw.Version > 0 {
		out.Version = raw.Version
	}
	if raw.Enabled != nil {
		out.Enabled = *raw.Enabled
	}
	if raw.Strict != nil {
		out.Strict = *raw.Strict
	}
	if len(raw.Models) > 0 {
		for k, v := range raw.Models {
			if k != "" && v != "" {
				out.Models[k] = v
			}
		}
	}
	if raw.AutoUpdate != nil {
		if raw.AutoUpdate.Enabled != nil {
			out.AutoUpdate.Enabled = *raw.AutoUpdate.Enabled
		}
		if raw.AutoUpdate.CheckIntervalHours != nil && *raw.AutoUpdate.CheckIntervalHours > 0 {
			out.AutoUpdate.CheckIntervalHours = *raw.AutoUpdate.CheckIntervalHours
		}
		if strings.TrimSpace(raw.AutoUpdate.Channel) != "" {
			out.AutoUpdate.Channel = strings.TrimSpace(raw.AutoUpdate.Channel)
		}
	}
	return out
}

// Save 写入用户配置，并同步运行时副本到资产目录。
func Save(home string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Models == nil {
		cfg.Models = Default().Models
	}
	if cfg.AutoUpdate.CheckIntervalHours <= 0 {
		cfg.AutoUpdate.CheckIntervalHours = DefaultAutoUpdateCheckIntervalHours
	}
	if strings.TrimSpace(cfg.AutoUpdate.Channel) == "" {
		cfg.AutoUpdate.Channel = DefaultAutoUpdateChannel
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	userPath := paths.UserConfigFile(home)
	if err := writeFile(userPath, payload); err != nil {
		return err
	}
	return writeFile(paths.RuntimeConfigFile(home), payload)
}

// SyncRuntime 把当前生效配置写到预加载可读的运行时路径。
func SyncRuntime(home string, cfg Config) error {
	if cfg.Models == nil {
		cfg.Models = Default().Models
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeFile(paths.RuntimeConfigFile(home), payload)
}

// SetModel 设置单个 Mode 的模型 ID。
func SetModel(home, mode, modelID string) (Config, error) {
	mode = NormalizeModeAlias(mode)
	if !IsValidMode(mode) {
		return Config{}, fmt.Errorf("非法 mode %q（允许：%s；ask 会映射为 search）", mode, strings.Join(ValidModes, ", "))
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return Config{}, errors.New("model-id 不能为空")
	}
	cfg := Load(home)
	if cfg.Models == nil {
		cfg.Models = Default().Models
	}
	cfg.Models[mode] = modelID
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetMany 批量设置 mode=model 对。
func SetMany(home string, pairs map[string]string) (Config, error) {
	cfg := Load(home)
	if cfg.Models == nil {
		cfg.Models = Default().Models
	}
	for mode, modelID := range pairs {
		mode = NormalizeModeAlias(mode)
		if !IsValidMode(mode) {
			return Config{}, fmt.Errorf("非法 mode %q（允许：%s）", mode, strings.Join(ValidModes, ", "))
		}
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return Config{}, fmt.Errorf("mode %s 的 model-id 不能为空", mode)
		}
		cfg.Models[mode] = modelID
	}
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetEnabled 开关自动切换。
func SetEnabled(home string, enabled bool) (Config, error) {
	cfg := Load(home)
	cfg.Enabled = enabled
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetStrict 设置严格模式。
func SetStrict(home string, strict bool) (Config, error) {
	cfg := Load(home)
	cfg.Strict = strict
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetAutoUpdateEnabled 开关静默自更新。
func SetAutoUpdateEnabled(home string, enabled bool) (Config, error) {
	cfg := Load(home)
	cfg.AutoUpdate.Enabled = enabled
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetAutoUpdateInterval 设置静默自更新检查间隔（小时）。
func SetAutoUpdateInterval(home string, hours int) (Config, error) {
	if hours <= 0 {
		return Config{}, errors.New("检查间隔必须大于 0 小时")
	}
	cfg := Load(home)
	cfg.AutoUpdate.CheckIntervalHours = hours
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Reset 恢复默认配置（覆盖用户文件）。
func Reset(home string) (Config, error) {
	cfg := Default()
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// GloballyDisabled 环境变量总开关为 0 时关闭。
func GloballyDisabled() bool {
	return os.Getenv(paths.EnvDisable) == "0"
}

func writeFile(path string, payload []byte) error {
	if path == "" {
		return errors.New("配置路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
