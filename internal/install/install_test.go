package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/x0c/agent-auto-model/internal/paths"
)

func TestStripPathBlock(t *testing.T) {
	in := "export FOO=1\n\n# cursor-mode-model PATH\n[ -f \"/tmp/path.sh\" ] && . \"/tmp/path.sh\"\n\nexport BAR=2\n"
	got := stripPathBlock(in)
	if strings.Contains(got, "agent-auto-model") || strings.Contains(got, "cursor-mode-model") {
		t.Fatalf("未清除: %q", got)
	}
	if !strings.Contains(got, "export FOO=1") || !strings.Contains(got, "export BAR=2") {
		t.Fatalf("误删其它行: %q", got)
	}
}

func TestUninstallRemovesShellHook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	self := filepath.Join(home, "agent-auto-model")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("export PREEXIST=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(home, self, false, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), pathMarker) {
		t.Fatalf("install 未写入 PATH: %s", data)
	}
	codexName := "codex"
	if runtime.GOOS == "windows" {
		codexName = "codex.cmd"
	}
	codexWrap := filepath.Join(paths.WrapperBinDir(home), codexName)
	if _, err := os.Stat(codexWrap); err != nil {
		t.Fatalf("install 未写入 codex 包装: %v", err)
	}
	if _, err := Uninstall(home, false, nil); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), pathMarker) || strings.Contains(string(data), "agent-auto-model") || strings.Contains(string(data), "cursor-mode-model") {
		t.Fatalf("uninstall 未清 PATH: %s", data)
	}
}

func TestInstallRuntimeFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	self := filepath.Join(home, "agent-auto-model")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(home, self, false, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	codexName, agentName := "codex", "agent"
	if runtime.GOOS == "windows" {
		codexName, agentName = "codex.cmd", "agent.cmd"
	}
	if _, err := os.Stat(filepath.Join(paths.WrapperBinDir(home), codexName)); err != nil {
		t.Fatalf("应写入 codex 包装: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.WrapperBinDir(home), agentName)); err == nil {
		t.Fatal("仅装 codex 时不应写入 agent 包装")
	}
	if _, err := Install(home, self, false, []string{"nope"}); err == nil {
		t.Fatal("非法 runtime 应报错")
	}
	if _, err := Uninstall(home, false, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.WrapperBinDir(home), codexName)); err == nil {
		t.Fatal("uninstall --runtime codex 应移除包装")
	}
}

func TestInstallRemovesLeftoverWrappers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	self := filepath.Join(home, "agent-auto-model")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	leftoverBin := filepath.Join(paths.LeftoverDataDir(home), "bin")
	if err := os.MkdirAll(leftoverBin, 0o755); err != nil {
		t.Fatal(err)
	}
	leftoverAgent := filepath.Join(leftoverBin, "agent")
	if err := os.WriteFile(leftoverAgent, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	leftoverCmd := filepath.Join(paths.LocalBinDir(home), paths.LeftoverCommandName())
	if err := os.MkdirAll(filepath.Dir(leftoverCmd), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leftoverCmd, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(home, self, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(leftoverAgent); !os.IsNotExist(err) {
		t.Fatalf("install 应清掉旧包装目录: %v", err)
	}
	if _, err := os.Stat(leftoverCmd); !os.IsNotExist(err) {
		t.Fatalf("install 应清掉旧命令名: %v", err)
	}
}
