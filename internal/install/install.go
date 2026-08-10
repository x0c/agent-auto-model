// Package install 写入配置、资产，并安装 PATH 前置包装。
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/assets"
	"github.com/x0c/cursor-mode-model/internal/config"
	"github.com/x0c/cursor-mode-model/internal/paths"
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
	if err := ensurePath(home); err != nil {
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
	dir := paths.WrapperBinDir(home)
	for _, name := range wrapperNames() {
		_ = os.Remove(filepath.Join(dir, name))
	}
	_ = removePath(home)
	st := State{Enabled: false}
	if prev, err := readState(home); err == nil {
		st.SelfBinary = prev.SelfBinary
	}
	_ = writeState(home, st)
	res.Status = "ok"
	return res, nil
}

func wrapperNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"agent.cmd", "cursor-agent.cmd"}
	}
	return []string{"agent", "cursor-agent"}
}

func writeWrappers(home, selfBinary string) error {
	dir := paths.WrapperBinDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		for _, name := range []string{"agent", "cursor-agent"} {
			body := fmt.Sprintf("@echo off\r\nREM 由 cursor-mode-model install 生成\r\n%q exec --invoked-as %s -- %%*\r\n", selfBinary, name)
			path := filepath.Join(dir, name+".cmd")
			if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
				return err
			}
		}
		return nil
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
	if runtime.GOOS == "windows" {
		return paths.WrapperBinDir(home)
	}
	return fmt.Sprintf(`export PATH=%q:$PATH`, paths.WrapperBinDir(home))
}

func ensurePath(home string) error {
	if runtime.GOOS == "windows" {
		return ensureWindowsUserPath(paths.WrapperBinDir(home))
	}
	return ensureUnixPathSnippet(home)
}

func removePath(home string) error {
	if runtime.GOOS == "windows" {
		return removeWindowsUserPath(paths.WrapperBinDir(home))
	}
	return removeUnixPathSnippets(home)
}

const pathMarker = "# cursor-mode-model PATH"

func ensureUnixPathSnippet(home string) error {
	snippetDir := filepath.Join(paths.DataDir(home), "shell")
	if err := os.MkdirAll(snippetDir, 0o755); err != nil {
		return err
	}
	snippet := filepath.Join(snippetDir, "path.sh")
	body := "# cursor-mode-model：让包装后的 agent 优先于官方入口\n" + pathExportLine(home) + "\n"
	if err := os.WriteFile(snippet, []byte(body), 0o644); err != nil {
		return err
	}
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
		if strings.Contains(text, pathMarker) {
			continue
		}
		add := "\n" + pathMarker + "\n" + line + "\n"
		if err := os.WriteFile(rc, []byte(text+add), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func removeUnixPathSnippets(home string) error {
	snippet := filepath.Join(paths.DataDir(home), "shell", "path.sh")
	_ = os.Remove(snippet)
	for _, rcName := range []string{".zshrc", ".bashrc", ".zprofile"} {
		rc := filepath.Join(home, rcName)
		data, err := os.ReadFile(rc)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		cleaned := stripPathBlock(string(data))
		if cleaned == string(data) {
			continue
		}
		if err := os.WriteFile(rc, []byte(cleaned), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func stripPathBlock(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	skipNext := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == pathMarker {
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			if strings.Contains(line, "cursor-mode-model") && strings.Contains(line, "path.sh") {
				continue
			}
			out = append(out, line)
			continue
		}
		out = append(out, line)
	}
	cleaned := strings.Join(out, "\n")
	for strings.HasSuffix(cleaned, "\n\n\n") {
		cleaned = strings.TrimSuffix(cleaned, "\n")
	}
	return cleaned
}

func ensureWindowsUserPath(dir string) error {
	return runPowerShellPathMutate(dir, false)
}

func removeWindowsUserPath(dir string) error {
	return runPowerShellPathMutate(dir, true)
}

func runPowerShellPathMutate(dir string, remove bool) error {
	// 用 PowerShell 改用户 PATH，避免 setx 截断。
	script := `
$ErrorActionPreference = 'Stop'
$dir = $env:CMM_PATH_DIR
$remove = $env:CMM_PATH_REMOVE -eq '1'
$trim = [char[]]'\/'
$normalized = $dir.TrimEnd($trim)
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$entries = @()
if ($userPath) { $entries = @($userPath -split ';' | Where-Object { $_ }) }
$filtered = @($entries | Where-Object {
  -not $_.Trim().TrimEnd($trim).Equals($normalized, [StringComparison]::OrdinalIgnoreCase)
})
if (-not $remove) {
  $filtered = @($normalized) + $filtered
}
$newPath = ($filtered -join ';')
[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(),
		"CMM_PATH_DIR="+dir,
		fmt.Sprintf("CMM_PATH_REMOVE=%d", boolTo01(remove)),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("更新用户 PATH 失败：%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func boolTo01(v bool) int {
	if v {
		return 1
	}
	return 0
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
