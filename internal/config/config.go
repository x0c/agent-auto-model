// Package config 读写 Mode→模型映射配置。
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

// DefaultModels 默认映射：Plan→Opus 5，其它→Grok 4.5。
var DefaultModels = map[string]string{
	"plan":    "claude-opus-5-thinking-high",
	"default": "cursor-grok-4.5-high-fast",
	"search":  "cursor-grok-4.5-high-fast",
	"debug":   "cursor-grok-4.5-high-fast",
}

// Config 用户可改配置。
type Config struct {
	Version int               `json:"version"`
	Enabled bool              `json:"enabled"`
	Strict  bool              `json:"strict"`
	Models  map[string]string `json:"models"`
}

// Default 返回默认配置副本。
func Default() Config {
	models := make(map[string]string, len(DefaultModels))
	for k, v := range DefaultModels {
		models[k] = v
	}
	return Config{Version: 1, Enabled: true, Strict: false, Models: models}
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
		Version int               `json:"version"`
		Enabled *bool             `json:"enabled"`
		Strict  *bool             `json:"strict"`
		Models  map[string]string `json:"models"`
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
