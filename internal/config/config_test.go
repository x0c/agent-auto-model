package config

import (
	"os"
	"path/filepath"
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
	rt := filepath.Join(home, ".local", "share", "cursor-mode-model", "assets", "config.json")
	if _, err := os.Stat(rt); err != nil {
		t.Fatalf("运行时配置未同步: %v", err)
	}
}

func TestLoadMissingEnabledDefaultsTrue(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "cursor-mode-model", "config.json")
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
}
