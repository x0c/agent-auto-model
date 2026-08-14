// Package assets 把内嵌预加载脚本落到本机数据目录。
package assets

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/paths"
)

//go:embed register.mjs
var registerMJS []byte

//go:embed anchors.json
var anchorsJSON []byte

// Ensure 写入 register.mjs / anchors.json，并确保运行时配置存在。
func Ensure(home string) (registerPath string, err error) {
	dir := paths.AssetsDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	registerPath = paths.RegisterFile(home)
	if err := writeIfChanged(registerPath, registerMJS); err != nil {
		return "", err
	}
	if err := writeIfChanged(filepath.Join(dir, "anchors.json"), anchorsJSON); err != nil {
		return "", err
	}
	runtimeCfg := paths.RuntimeConfigFile(home)
	if _, err := os.Stat(runtimeCfg); err != nil {
		if err := config.SyncRuntime(home, config.LoadEffective(home)); err != nil {
			return "", err
		}
	}
	return registerPath, nil
}

// AnchorsJSON 返回内嵌锚点清单（供 anchors 包单一来源读取）。
func AnchorsJSON() []byte {
	return anchorsJSON
}

func writeIfChanged(path string, payload []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == string(payload) {
			return nil
		}
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
