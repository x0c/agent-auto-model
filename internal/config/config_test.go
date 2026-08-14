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
