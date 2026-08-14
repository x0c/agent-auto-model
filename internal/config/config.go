// Package config 读写 Mode→模型映射配置。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/recommended"
	"github.com/x0c/agent-auto-model/internal/runtime/codex/spec"
)

const (
	DefaultAutoUpdateChannel            = "github_release"
	DefaultAutoUpdateCheckIntervalHours = 24

	RuntimeCursor = "cursor"
	RuntimeCodex  = "codex"

	ModelsSourceRecommended = "recommended"
	ModelsSourceLocal       = "local"
)

// DefaultCursorModels 来自内置推荐配置：Plan→Opus 5，其它→当前最新 Grok high。
var DefaultCursorModels = map[string]string{}

// DefaultCodexModels 来自内置推荐配置。
var DefaultCodexModels = map[string]string{}

// ValidCursorModes 可配置的 Cursor Mode 键（CLI --mode ask 对应内部 search）。
var ValidCursorModes = []string{"plan", "default", "search", "debug"}

// ValidCodexModes Codex collaborationMode 仅 plan / default。
var ValidCodexModes = []string{"plan", "default"}

// ValidRuntimes 已接入的 Agent runtime。
var ValidRuntimes = []string{RuntimeCursor, RuntimeCodex}

// DefaultModels 兼容旧调用：等于 Cursor 默认映射。
var DefaultModels = DefaultCursorModels

// ValidModes 兼容旧调用：等于 Cursor modes。
var ValidModes = ValidCursorModes

func init() {
	emb := recommended.Embedded()
	DefaultCursorModels = emb.Models(RuntimeCursor)
	DefaultCodexModels = emb.Models(RuntimeCodex)
	DefaultModels = DefaultCursorModels
}

// Config 用户可改配置。
type Config struct {
	Version      int                      `json:"version"`
	Enabled      bool                     `json:"enabled"`
	Strict       bool                     `json:"strict"`
	ModelsSource string                   `json:"models_source"`
	Models       map[string]string        `json:"models,omitempty"`
	Runtimes     map[string]RuntimeConfig `json:"runtimes"`
	AutoUpdate   AutoUpdateConfig         `json:"auto_update"`
}

// RuntimeConfig 单个 Agent runtime 的开关与映射。
type RuntimeConfig struct {
	Enabled bool              `json:"enabled"`
	Models  map[string]string `json:"models"`
}

// AutoUpdateConfig 控制静默自更新。
type AutoUpdateConfig struct {
	Enabled            bool   `json:"enabled"`
	CheckIntervalHours int    `json:"check_interval_hours"`
	Channel            string `json:"channel"`
}

// Default 返回默认配置副本。
func Default() Config {
	return Config{
		Version:      2,
		Enabled:      true,
		Strict:       false,
		ModelsSource: ModelsSourceRecommended,
		Models:       copyMap(DefaultCursorModels),
		Runtimes: map[string]RuntimeConfig{
			RuntimeCursor: {Enabled: true, Models: copyMap(DefaultCursorModels)},
			RuntimeCodex:  {Enabled: true, Models: copyMap(DefaultCodexModels)},
		},
		AutoUpdate: AutoUpdateConfig{
			Enabled:            true,
			CheckIntervalHours: DefaultAutoUpdateCheckIntervalHours,
			Channel:            DefaultAutoUpdateChannel,
		},
	}
}

// IsValidRuntime 判断 runtime 名是否合法。
func IsValidRuntime(name string) bool {
	for _, r := range ValidRuntimes {
		if r == name {
			return true
		}
	}
	return false
}

// ValidModesFor 返回 runtime 允许的 mode 键。
func ValidModesFor(runtime string) []string {
	switch runtime {
	case RuntimeCodex:
		return append([]string{}, ValidCodexModes...)
	default:
		return append([]string{}, ValidCursorModes...)
	}
}

// IsValidMode 判断 Cursor mode 键是否合法。
func IsValidMode(mode string) bool {
	return isModeOf(RuntimeCursor, mode)
}

func isModeOf(runtime, mode string) bool {
	for _, m := range ValidModesFor(runtime) {
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

// ParseTarget 解析 "plan" / "cursor.plan" / "codex.default"。
func ParseTarget(raw string) (runtime, mode string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("mode 不能为空")
	}
	if runtimeName, rest, ok := strings.Cut(raw, "."); ok {
		runtimeName = strings.ToLower(strings.TrimSpace(runtimeName))
		mode = NormalizeModeAlias(rest)
		if !IsValidRuntime(runtimeName) {
			return "", "", fmt.Errorf("非法 runtime %q（允许：%s）", runtimeName, strings.Join(ValidRuntimes, ", "))
		}
		if !isModeOf(runtimeName, mode) {
			return "", "", fmt.Errorf("非法 mode %q（runtime %s 允许：%s）", mode, runtimeName, strings.Join(ValidModesFor(runtimeName), ", "))
		}
		return runtimeName, mode, nil
	}
	mode = NormalizeModeAlias(raw)
	if !isModeOf(RuntimeCursor, mode) {
		return "", "", fmt.Errorf("非法 mode %q（允许：%s；或使用 runtime.mode，如 codex.plan）", raw, strings.Join(ValidCursorModes, ", "))
	}
	return RuntimeCursor, mode, nil
}

// Load 读取用户配置；不存在或损坏时返回默认。
// v1 扁平 models 或旧目录下的配置会升级为 v2 并写到新路径。
func Load(home string) Config {
	path := paths.UserConfigFile(home)
	data, err := os.ReadFile(path)
	fromLegacy := false
	if err != nil {
		leftover := paths.LeftoverUserConfigFile(home)
		data, err = os.ReadFile(leftover)
		if err != nil {
			return Default()
		}
		fromLegacy = true
	}
	cfg, ok, inferred := parseConfig(data)
	if !ok {
		return Default()
	}
	var head struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(data, &head)
	if fromLegacy || head.Version < 2 || inferred {
		_ = Save(home, cfg)
	}
	return cfg
}

func parseConfig(data []byte) (Config, bool, bool) {
	var raw struct {
		Version      int               `json:"version"`
		Enabled      *bool             `json:"enabled"`
		Strict       *bool             `json:"strict"`
		ModelsSource string            `json:"models_source"`
		Models       map[string]string `json:"models"`
		Runtimes     map[string]*struct {
			Enabled *bool             `json:"enabled"`
			Models  map[string]string `json:"models"`
		} `json:"runtimes"`
		AutoUpdate *struct {
			Enabled            *bool  `json:"enabled"`
			CheckIntervalHours *int   `json:"check_interval_hours"`
			Channel            string `json:"channel"`
		} `json:"auto_update"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, false, false
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
				if out.Runtimes[RuntimeCursor].Models == nil {
					rt := out.Runtimes[RuntimeCursor]
					rt.Models = map[string]string{}
					out.Runtimes[RuntimeCursor] = rt
				}
				out.Runtimes[RuntimeCursor].Models[k] = v
			}
		}
	}
	if raw.Runtimes != nil {
		for name, rt := range raw.Runtimes {
			if rt == nil {
				continue
			}
			cur := out.Runtimes[name]
			if cur.Models == nil {
				cur.Models = map[string]string{}
			}
			if rt.Enabled != nil {
				cur.Enabled = *rt.Enabled
			}
			for k, v := range rt.Models {
				if k != "" && v != "" {
					cur.Models[k] = v
				}
			}
			out.Runtimes[name] = cur
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
	normalize(&out)
	inferred := false
	if strings.TrimSpace(raw.ModelsSource) == "" {
		out.ModelsSource = inferModelsSource(out)
		inferred = true
	} else {
		src, err := ParseModelsSource(raw.ModelsSource)
		if err != nil {
			out.ModelsSource = inferModelsSource(out)
			inferred = true
		} else {
			out.ModelsSource = src
		}
	}
	return out, true, inferred
}

func normalize(cfg *Config) {
	if cfg.Version < 2 {
		cfg.Version = 2
	}
	if cfg.Runtimes == nil {
		cfg.Runtimes = map[string]RuntimeConfig{}
	}
	cursor := cfg.Runtimes[RuntimeCursor]
	if cursor.Models == nil {
		cursor.Models = copyMap(DefaultCursorModels)
	}
	if len(cfg.Models) > 0 {
		for k, v := range cfg.Models {
			if k != "" && v != "" {
				cursor.Models[k] = v
			}
		}
	}
	cfg.Runtimes[RuntimeCursor] = cursor
	codex := cfg.Runtimes[RuntimeCodex]
	if codex.Models == nil {
		codex.Models = copyMap(DefaultCodexModels)
	}
	cfg.Runtimes[RuntimeCodex] = codex
	cfg.Models = copyMap(cfg.Runtimes[RuntimeCursor].Models)
	if cfg.AutoUpdate.CheckIntervalHours <= 0 {
		cfg.AutoUpdate.CheckIntervalHours = DefaultAutoUpdateCheckIntervalHours
	}
	if strings.TrimSpace(cfg.AutoUpdate.Channel) == "" {
		cfg.AutoUpdate.Channel = DefaultAutoUpdateChannel
	}
	if src, err := ParseModelsSource(cfg.ModelsSource); err == nil {
		cfg.ModelsSource = src
	} else if cfg.ModelsSource == "" {
		cfg.ModelsSource = ModelsSourceRecommended
	}
}

// Save 写入用户配置，并同步运行时副本到资产目录。
func Save(home string, cfg Config) error {
	normalize(&cfg)
	payload, err := marshal(cfg)
	if err != nil {
		return err
	}
	if err := writeFile(paths.UserConfigFile(home), payload); err != nil {
		return err
	}
	return writeFile(paths.RuntimeConfigFile(home), payload)
}

// SyncRuntime 把当前生效配置写到预加载可读的运行时路径。
func SyncRuntime(home string, cfg Config) error {
	normalize(&cfg)
	payload, err := marshal(cfg)
	if err != nil {
		return err
	}
	return writeFile(paths.RuntimeConfigFile(home), payload)
}

func marshal(cfg Config) ([]byte, error) {
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

// ModelsFor 返回指定 runtime 的映射。
func ModelsFor(cfg Config, runtime string) map[string]string {
	if cfg.Runtimes == nil {
		if runtime == RuntimeCursor {
			return copyMap(cfg.Models)
		}
		return map[string]string{}
	}
	rt, ok := cfg.Runtimes[runtime]
	if !ok || rt.Models == nil {
		if runtime == RuntimeCursor {
			return copyMap(cfg.Models)
		}
		return map[string]string{}
	}
	return copyMap(rt.Models)
}

// RuntimeEnabled 总开关与 runtime 开关都开时才生效。
func RuntimeEnabled(cfg Config, runtime string) bool {
	if !cfg.Enabled {
		return false
	}
	rt, ok := cfg.Runtimes[runtime]
	if !ok {
		return runtime == RuntimeCursor
	}
	return rt.Enabled
}

// SetModel 设置单个 Mode 的模型 ID（默认 Cursor）。
func SetModel(home, mode, modelID string) (Config, error) {
	return SetRuntimeModel(home, RuntimeCursor, mode, modelID)
}

// SetRuntimeModel 设置指定 runtime 的 Mode 映射。
func SetRuntimeModel(home, runtime, mode, modelID string) (Config, error) {
	if runtime == "" {
		runtime = RuntimeCursor
	}
	if !IsValidRuntime(runtime) {
		return Config{}, fmt.Errorf("非法 runtime %q（允许：%s）", runtime, strings.Join(ValidRuntimes, ", "))
	}
	mode = NormalizeModeAlias(mode)
	if !isModeOf(runtime, mode) {
		return Config{}, fmt.Errorf("非法 mode %q（runtime %s 允许：%s）", mode, runtime, strings.Join(ValidModesFor(runtime), ", "))
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return Config{}, errors.New("model-id 不能为空")
	}
	if err := validateModelID(runtime, modelID); err != nil {
		return Config{}, err
	}
	cfg := Load(home)
	if cfg.ModelsSource == ModelsSourceRecommended {
		cfg = snapshotRecommendedToLocal(home, cfg)
	}
	rt := cfg.Runtimes[runtime]
	if rt.Models == nil {
		rt.Models = map[string]string{}
	}
	rt.Models[mode] = modelID
	cfg.Runtimes[runtime] = rt
	if runtime == RuntimeCursor {
		if cfg.Models == nil {
			cfg.Models = map[string]string{}
		}
		cfg.Models[mode] = modelID
	}
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetMany 批量设置 mode=model 或 runtime.mode=model 对。
func SetMany(home string, pairs map[string]string) (Config, error) {
	cfg := Load(home)
	if cfg.ModelsSource == ModelsSourceRecommended {
		cfg = snapshotRecommendedToLocal(home, cfg)
	}
	for raw, modelID := range pairs {
		runtime, mode, err := ParseTarget(raw)
		if err != nil {
			return Config{}, err
		}
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return Config{}, fmt.Errorf("mode %s 的 model-id 不能为空", raw)
		}
		if err := validateModelID(runtime, modelID); err != nil {
			return Config{}, err
		}
		rt := cfg.Runtimes[runtime]
		if rt.Models == nil {
			rt.Models = map[string]string{}
		}
		rt.Models[mode] = modelID
		cfg.Runtimes[runtime] = rt
		if runtime == RuntimeCursor {
			if cfg.Models == nil {
				cfg.Models = map[string]string{}
			}
			cfg.Models[mode] = modelID
		}
	}
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetEnabled 开关自动切换（总开关）。
func SetEnabled(home string, enabled bool) (Config, error) {
	cfg := Load(home)
	cfg.Enabled = enabled
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SetRuntimeEnabled 开关单个 runtime。
func SetRuntimeEnabled(home, runtime string, enabled bool) (Config, error) {
	if !IsValidRuntime(runtime) {
		return Config{}, fmt.Errorf("非法 runtime %q（允许：%s）", runtime, strings.Join(ValidRuntimes, ", "))
	}
	cfg := Load(home)
	rt := cfg.Runtimes[runtime]
	rt.Enabled = enabled
	cfg.Runtimes[runtime] = rt
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
	v := os.Getenv(paths.EnvDisable)
	return v == "0"
}

// ParseModelsSource 解析 recommended / local（及其中文叫法）。
func ParseModelsSource(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case ModelsSourceRecommended, "推荐配置":
		return ModelsSourceRecommended, nil
	case ModelsSourceLocal, "本地自定义":
		return ModelsSourceLocal, nil
	default:
		return "", fmt.Errorf("非法模型映射来源 %q（允许：recommended 或 local）", raw)
	}
}

// ModelsSourceTag 来源的中文展示名。
func ModelsSourceTag(source string) string {
	if source == ModelsSourceLocal {
		return "本地自定义"
	}
	return "推荐配置"
}

// ApplyRecommended 在来源为推荐配置时，用本机缓存/内置覆盖模型映射。
func ApplyRecommended(home string, cfg Config) Config {
	if cfg.ModelsSource != ModelsSourceRecommended {
		return cfg
	}
	return overlayRecommended(home, cfg)
}

// LoadEffective 读取用户配置并套上当前生效的模型映射。
func LoadEffective(home string) Config {
	return ApplyRecommended(home, Load(home))
}

// MatchesRecommended 本地存储的映射是否与当前推荐一致。
func MatchesRecommended(home string, cfg Config) bool {
	return matchesFile(cfg, recommended.Resolve(home))
}

// SetModelsSource 切换模型映射来源。切到本地自定义时先拍下当前推荐映射。
func SetModelsSource(home, source string) (Config, error) {
	src, err := ParseModelsSource(source)
	if err != nil {
		return Config{}, err
	}
	cfg := Load(home)
	if src == ModelsSourceLocal && cfg.ModelsSource != ModelsSourceLocal {
		cfg = snapshotRecommendedToLocal(home, cfg)
	} else {
		cfg.ModelsSource = src
	}
	if err := Save(home, cfg); err != nil {
		return Config{}, err
	}
	return ApplyRecommended(home, cfg), nil
}

func snapshotRecommendedToLocal(home string, cfg Config) Config {
	cfg = overlayRecommended(home, cfg)
	cfg.ModelsSource = ModelsSourceLocal
	return cfg
}

func overlayRecommended(home string, cfg Config) Config {
	rec := recommended.Resolve(home)
	if cfg.Runtimes == nil {
		cfg.Runtimes = map[string]RuntimeConfig{}
	}
	for _, name := range ValidRuntimes {
		rt := cfg.Runtimes[name]
		rt.Models = rec.Models(name)
		cfg.Runtimes[name] = rt
	}
	cfg.Models = copyMap(cfg.Runtimes[RuntimeCursor].Models)
	return cfg
}

func inferModelsSource(cfg Config) string {
	if matchesFile(cfg, recommended.Embedded()) {
		return ModelsSourceRecommended
	}
	return ModelsSourceLocal
}

func matchesFile(cfg Config, f recommended.File) bool {
	for _, name := range ValidRuntimes {
		want := f.Models(name)
		got := ModelsFor(cfg, name)
		for _, mode := range ValidModesFor(name) {
			if strings.TrimSpace(got[mode]) != strings.TrimSpace(want[mode]) {
				return false
			}
		}
	}
	return true
}

func validateModelID(runtime, modelID string) error {
	if runtime != RuntimeCodex {
		return nil
	}
	s := spec.Parse(modelID)
	if s.Empty() {
		return errors.New("Codex 模型规格不能为空")
	}
	if !spec.ValidEffort(s.Effort) {
		return fmt.Errorf("非法 Codex effort %q（允许：low|medium|high|xhigh|max|ultra，或省略）", s.Effort)
	}
	return nil
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
