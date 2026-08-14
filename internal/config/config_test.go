package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAndSaveLoad(t *testing.T) {
	home := t.TempDir()
	cfg := Default()
	if cfg.Models["plan"] == "" || cfg.Models["default"] == "" {
		t.Fatal("默认 models 不完整")
	}
	if !cfg.AutoUpdate.Enabled || cfg.AutoUpdate.CheckIntervalHours <= 0 {
		t.Fatalf("默认 auto_update 不完整: %#v", cfg.AutoUpdate)
	}
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.Models["plan"] != cfg.Models["plan"] {
		t.Fatalf("plan=%s", got.Models["plan"])
	}
	if got.AutoUpdate.Channel != DefaultAutoUpdateChannel {
		t.Fatalf("channel=%s", got.AutoUpdate.Channel)
	}
	rt := filepath.Join(home, ".local", "share", "agent-auto-model", "assets", "config.json")
	if _, err := os.Stat(rt); err != nil {
		t.Fatalf("运行时配置未同步: %v", err)
	}
}

func TestLoadMissingEnabledDefaultsTrue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	path := filepath.Join(home, ".config", "agent-auto-model", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"models":{"plan":"x","default":"y"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if !got.Enabled {
		t.Fatal("缺省 enabled 应视为 true")
	}
}

func TestSetModelAndAliases(t *testing.T) {
	home := t.TempDir()
	cfg, err := SetModel(home, "ask", "cursor-grok-*-high")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Models["search"] != "cursor-grok-*-high" {
		t.Fatalf("ask 未映射到 search: %#v", cfg.Models)
	}
	if _, err := SetModel(home, "nope", "x"); err == nil {
		t.Fatal("非法 mode 应报错")
	}
}

func TestSetManyEnableStrictReset(t *testing.T) {
	home := t.TempDir()
	cfg, err := SetMany(home, map[string]string{
		"plan":    "claude-opus-5-thinking-high",
		"default": "cursor-grok-*-high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Models["plan"] != "claude-opus-5-thinking-high" {
		t.Fatal(cfg.Models["plan"])
	}
	cfg, err = SetEnabled(home, false)
	if err != nil || cfg.Enabled {
		t.Fatalf("enable: %#v %v", cfg, err)
	}
	cfg, err = SetStrict(home, true)
	if err != nil || !cfg.Strict {
		t.Fatalf("strict: %#v %v", cfg, err)
	}
	cfg, err = SetAutoUpdateEnabled(home, false)
	if err != nil || cfg.AutoUpdate.Enabled {
		t.Fatalf("auto update enabled: %#v %v", cfg, err)
	}
	cfg, err = SetAutoUpdateInterval(home, 12)
	if err != nil || cfg.AutoUpdate.CheckIntervalHours != 12 {
		t.Fatalf("auto update interval: %#v %v", cfg, err)
	}
	cfg, err = Reset(home)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Strict || !cfg.AutoUpdate.Enabled {
		t.Fatalf("reset 后应为默认: %#v", cfg)
	}
	if cfg.Models["plan"] != DefaultModels["plan"] {
		t.Fatal(cfg.Models["plan"])
	}
	if cfg.Runtimes[RuntimeCodex].Models["plan"] != DefaultCodexModels["plan"] {
		t.Fatalf("codex plan=%s", cfg.Runtimes[RuntimeCodex].Models["plan"])
	}
}

func TestLoadV1PromotesToCursorRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	path := filepath.Join(home, ".config", "agent-auto-model", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"enabled":true,"models":{"plan":"opus-x","default":"grok-y"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.Runtimes[RuntimeCursor].Models["plan"] != "opus-x" {
		t.Fatalf("%#v", got.Runtimes[RuntimeCursor].Models)
	}
	if got.Runtimes[RuntimeCodex].Models["plan"] != DefaultCodexModels["plan"] {
		t.Fatalf("codex defaults lost: %#v", got.Runtimes[RuntimeCodex].Models)
	}
	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rewritten), `"version": 2`) {
		t.Fatalf("v1 应写回 v2: %s", rewritten)
	}
	if !strings.Contains(string(rewritten), `"codex"`) {
		t.Fatalf("写回应含 codex runtime: %s", rewritten)
	}
}

func TestLoadMigratesLegacyConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	legacy := filepath.Join(home, "cfg", "cursor-mode-model", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"enabled":true,"models":{"plan":"opus-legacy"}}`
	if err := os.WriteFile(legacy, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.Runtimes[RuntimeCursor].Models["plan"] != "opus-legacy" {
		t.Fatalf("%#v", got.Runtimes[RuntimeCursor].Models)
	}
	if _, err := os.Stat(filepath.Join(home, "cfg", "agent-auto-model", "config.json")); err != nil {
		t.Fatalf("应迁移到新配置目录: %v", err)
	}
}

func TestSetRuntimeModelRejectsBadEffort(t *testing.T) {
	home := t.TempDir()
	if _, err := SetRuntimeModel(home, RuntimeCodex, "plan", "gpt-5.6-sol:nope"); err == nil {
		t.Fatal("非法 effort 应报错")
	}
}

func TestParseTarget(t *testing.T) {
	rt, mode, err := ParseTarget("codex.plan")
	if err != nil || rt != RuntimeCodex || mode != "plan" {
		t.Fatalf("%s %s %v", rt, mode, err)
	}
	rt, mode, err = ParseTarget("ask")
	if err != nil || rt != RuntimeCursor || mode != "search" {
		t.Fatalf("%s %s %v", rt, mode, err)
	}
	if _, _, err := ParseTarget("codex.search"); err == nil {
		t.Fatal("codex.search 应非法")
	}
}

func TestSetRuntimeModel(t *testing.T) {
	home := t.TempDir()
	cfg, err := SetRuntimeModel(home, RuntimeCodex, "plan", "gpt-5.6-sol:high")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtimes[RuntimeCodex].Models["plan"] != "gpt-5.6-sol:high" {
		t.Fatalf("%#v", cfg.Runtimes[RuntimeCodex].Models)
	}
}

func TestDefaultModelsSourceIsRecommended(t *testing.T) {
	if Default().ModelsSource != ModelsSourceRecommended {
		t.Fatalf("%s", Default().ModelsSource)
	}
}

func TestInferOldDefaultAsRecommended(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cfg := Default()
	cfg.ModelsSource = ""
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	// 去掉来源字段，模拟升级前的配置。
	path := filepath.Join(home, ".config", "agent-auto-model", "config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw), `"models_source": "recommended",`, "", 1)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.ModelsSource != ModelsSourceRecommended {
		t.Fatalf("与内置推荐一致时应判为推荐配置: %s", got.ModelsSource)
	}
}

func TestInferCustomAsLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cfg := Default()
	cfg.ModelsSource = ""
	rt := cfg.Runtimes[RuntimeCursor]
	rt.Models["plan"] = "my-opus"
	cfg.Runtimes[RuntimeCursor] = rt
	cfg.Models["plan"] = "my-opus"
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "agent-auto-model", "config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(string(raw), `"models_source": "recommended",`, "")
	body = strings.ReplaceAll(body, `"models_source": "local",`, "")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.ModelsSource != ModelsSourceLocal {
		t.Fatalf("改过映射应判为本地自定义: %s plan=%s", got.ModelsSource, got.Runtimes[RuntimeCursor].Models["plan"])
	}
}

func TestSetSwitchesRecommendedToLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if err := Save(home, Default()); err != nil {
		t.Fatal(err)
	}
	cfg, err := SetRuntimeModel(home, RuntimeCursor, "plan", "claude-opus-5-thinking-high")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelsSource != ModelsSourceLocal {
		t.Fatalf("改映射后应切成本地自定义: %s", cfg.ModelsSource)
	}
}

func TestSetModelsSourceRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if _, err := SetRuntimeModel(home, RuntimeCursor, "plan", "custom-plan-model"); err != nil {
		t.Fatal(err)
	}
	if Load(home).ModelsSource != ModelsSourceLocal {
		t.Fatal("应已是本地自定义")
	}
	cfg, err := SetModelsSource(home, ModelsSourceRecommended)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelsSource != ModelsSourceRecommended {
		t.Fatal(cfg.ModelsSource)
	}
	if cfg.Runtimes[RuntimeCursor].Models["plan"] != DefaultCursorModels["plan"] {
		t.Fatalf("切回推荐后生效映射应是推荐: %s", cfg.Runtimes[RuntimeCursor].Models["plan"])
	}
	cfg, err = SetModelsSource(home, ModelsSourceLocal)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelsSource != ModelsSourceLocal {
		t.Fatal(cfg.ModelsSource)
	}
	if cfg.Runtimes[RuntimeCursor].Models["plan"] != DefaultCursorModels["plan"] {
		t.Fatalf("切到本地时应拍下当时的推荐: %s", cfg.Runtimes[RuntimeCursor].Models["plan"])
	}
}

func TestLoadEffectiveUsesRecommended(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	cfg := Default()
	cfg.Runtimes[RuntimeCursor].Models["plan"] = "stale-local"
	cfg.Models["plan"] = "stale-local"
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	got := LoadEffective(home)
	if got.Runtimes[RuntimeCursor].Models["plan"] != DefaultCursorModels["plan"] {
		t.Fatalf("推荐来源应覆盖本地快照: %s", got.Runtimes[RuntimeCursor].Models["plan"])
	}
}
