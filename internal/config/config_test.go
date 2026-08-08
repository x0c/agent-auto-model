package config

import (
	"os"
	"path/filepath"
	"testing"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	cfg := Default()
	cfg.Models["plan"] = "custom-plan"
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.Models["plan"] != "custom-plan" {
		t.Fatalf("%#v", got.Models)
	}
	if !got.Enabled {
		t.Fatal("enabled")
	}
}

func TestLoadOmittingEnabledDefaultsTrue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	path := paths.UserConfigFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"models":{"plan":"x","default":"y"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if !got.Enabled {
		t.Fatal("缺省 enabled 应视为开启")
	}
	if got.Models["plan"] != "x" || got.Models["default"] != "y" {
		t.Fatalf("%#v", got.Models)
	}
}

func TestLoadEnabledFalseHonored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	cfg := Default()
	cfg.Enabled = false
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	got := Load(home)
	if got.Enabled {
		t.Fatal("enabled=false 应保留")
	}
}
