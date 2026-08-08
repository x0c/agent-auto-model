package config

import (
	"path/filepath"
	"testing"
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
