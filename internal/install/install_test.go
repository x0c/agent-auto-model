package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripPathBlock(t *testing.T) {
	in := "export FOO=1\n\n# cursor-mode-model PATH\n[ -f \"/tmp/path.sh\" ] && . \"/tmp/path.sh\"\n\nexport BAR=2\n"
	got := stripPathBlock(in)
	if strings.Contains(got, "cursor-mode-model") {
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
	self := filepath.Join(home, "cursor-mode-model")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("export PREEXIST=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(home, self, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), pathMarker) {
		t.Fatalf("install 未写入 PATH: %s", data)
	}
	if _, err := Uninstall(home, false); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), pathMarker) || strings.Contains(string(data), "cursor-mode-model") {
		t.Fatalf("uninstall 未清 PATH: %s", data)
	}
}
