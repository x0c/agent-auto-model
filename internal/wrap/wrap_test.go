package wrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/config"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

func TestPrepareEnvInjectsImport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CURSOR_MODE_MODEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	_ = os.Unsetenv("CURSOR_MODE_MODEL")
	_ = os.Unsetenv("NODE_OPTIONS")

	if err := config.Save(home, config.Default()); err != nil {
		t.Fatal(err)
	}
	env, err := PrepareEnv(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env["NODE_OPTIONS"], "--import=") {
		t.Fatalf("NODE_OPTIONS=%q", env["NODE_OPTIONS"])
	}
	if env[paths.EnvConfig] == "" {
		t.Fatal("缺少配置路径")
	}
	if _, err := os.Stat(env[paths.EnvConfig]); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareEnvDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CURSOR_MODE_MODEL", "0")
	env, err := PrepareEnv(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Fatalf("应为空: %#v", env)
	}
}

func TestPrepareEnvLocksOnExplicitModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CURSOR_MODE_MODEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	_ = os.Unsetenv("CURSOR_MODE_MODEL")
	if err := config.Save(home, config.Default()); err != nil {
		t.Fatal(err)
	}
	env, err := PrepareEnv(home, []string{"--model", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if env[paths.EnvLock] != "1" {
		t.Fatalf("应加锁: %#v", env)
	}
}

func TestAppendFlagIdempotent(t *testing.T) {
	got := appendFlag("--import=/a/register.mjs", "--import=/a/register.mjs")
	if got != "--import=/a/register.mjs" {
		t.Fatal(got)
	}
}
