// Package install 写入配置、资产，并安装 PATH 前置包装。
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/assets"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/config"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

// State 记录本工具自身二进制位置，供包装脚本调用。
type State struct {
	SelfBinary string `json:"self_binary"`
	Enabled    bool   `json:"enabled"`
}

// Result 安装/卸载结果摘要。
type Result struct {
	Status     string `json:"status"`
	UserConfig string `json:"user_config"`
	Register   string `json:"register"`
	WrapperBin string `json:"wrapper_bin"`
	Enabled    bool   `json:"enabled"`
	PathHint   string `json:"path_hint,omitempty"`
}

// Install 启用配置并安装 agent / cursor-agent 包装。
func Install(home, selfBinary string, dryRun bool) (Result, error) {
	res := Result{
		UserConfig: paths.UserConfigFile(home),
		Register:   paths.RegisterFile(home),
		WrapperBin: paths.WrapperBinDir(home),
		Enabled:    true,
		PathHint:   pathExportLine(home),
	}
	if dryRun {
		res.Status = "dry_run"
		return res, nil
	}
	cfg := config.Load(home)
	cfg.Enabled = true
	if err := config.Save(home, cfg); err != nil {
		return res, err
	}
	register, err := assets.Ensure(home)
	if err != nil {
		return res, err
	}
	res.Register = register
	absSelf, err := filepath.Abs(selfBinary)
	if err != nil {
		return res, err
	}
	if err := writeState(home, State{SelfBinary: absSelf, Enabled: true}); err != nil {
		return res, err
	}
	if err := writeWrappers(home, absSelf); err != nil {
		return res, err
	}
	if err := ensurePathSnippet(home); err != nil {
		return res, err
	}
	res.Status = "ok"
	return res, nil
}

// Uninstall 关闭自动切换并移除包装（配置文件可保留）。
func Uninstall(home string, dryRun bool) (Result, error) {
	res := Result{
		UserConfig: paths.UserConfigFile(home),
		Register:   paths.RegisterFile(home),
		WrapperBin: paths.WrapperBinDir(home),
		Enabled:    false,
	}
	if dryRun {
		res.Status = "dry_run"
		return res, nil
	}
	cfg := config.Load(home)
	cfg.Enabled = false
	if err := config.Save(home, cfg); err != nil {
		return res, err
	}
	_ = os.Remove(filepath.Join(paths.WrapperBinDir(home), "agent"))
	_ = os.Remove(filepath.Join(paths.WrapperBinDir(home), "cursor-agent"))
	st := State{Enabled: false}
	if prev, err := readState(home); err == nil {
		st.SelfBinary = prev.SelfBinary
	}
	_ = writeState(home, st)
	res.Status = "ok"
	return res, nil
}

func writeWrappers(home, selfBinary string) error {
	dir := paths.WrapperBinDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"agent", "cursor-agent"} {
		body := fmt.Sprintf(`#!/bin/sh
# 由 cursor-mode-model install 生成：注入 Mode→模型预加载后转调官方 Agent。
exec %q exec --invoked-as %q -- "$@"
`, selfBinary, name)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func pathExportLine(home string) string {
	return fmt.Sprintf(`export PATH=%q:$PATH`, paths.WrapperBinDir(home))
}

func ensurePathSnippet(home string) error {
	snippetDir := filepath.Join(paths.DataDir(home), "shell")
	if err := os.MkdirAll(snippetDir, 0o755); err != nil {
		return err
	}
	snippet := filepath.Join(snippetDir, "path.sh")
	body := "# cursor-mode-model：让包装后的 agent 优先于官方入口\n" + pathExportLine(home) + "\n"
	if err := os.WriteFile(snippet, []byte(body), 0o644); err != nil {
		return err
	}
	// 追加到常见 shell rc（幂等）
	marker := "# cursor-mode-model PATH"
	line := fmt.Sprintf(`[ -f %q ] && . %q`, snippet, snippet)
	for _, rcName := range []string{".zshrc", ".bashrc", ".zprofile"} {
		rc := filepath.Join(home, rcName)
		data, err := os.ReadFile(rc)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		text := string(data)
		if strings.Contains(text, marker) {
			continue
		}
		add := "\n" + marker + "\n" + line + "\n"
		if err := os.WriteFile(rc, []byte(text+add), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeState(home string, st State) error {
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := paths.StateFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readState(home string) (State, error) {
	var st State
	data, err := os.ReadFile(paths.StateFile(home))
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(data, &st)
	return st, err
}
