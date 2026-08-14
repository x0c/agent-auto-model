// Package install 写入配置、资产，并安装 PATH 前置包装。
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"github.com/x0c/agent-auto-model/internal/assets"
	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/recommended"
	aamruntime "github.com/x0c/agent-auto-model/internal/runtime"
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

// Install 启用配置并安装包装。runtimeNames 为空表示全部 runtime。
func Install(home, selfBinary string, dryRun bool, runtimeNames []string) (Result, error) {
	res := Result{
		UserConfig: paths.UserConfigFile(home),
		Register:   paths.RegisterFile(home),
		WrapperBin: paths.WrapperBinDir(home),
		Enabled:    true,
		PathHint:   pathExportLine(home),
	}
	infos, err := selectedRuntimes(runtimeNames)
	if err != nil {
		return res, err
	}
	if dryRun {
		res.Status = "dry_run"
		return res, nil
	}
	cfg := config.Load(home)
	cfg.Enabled = true
	if len(runtimeNames) > 0 {
		for _, info := range infos {
			rt := cfg.Runtimes[info.Name]
			rt.Enabled = true
			cfg.Runtimes[info.Name] = rt
		}
	}
	if err := config.Save(home, cfg); err != nil {
		return res, err
	}
	recommended.RefreshAtInstall(home)
	if err := config.SyncRuntime(home, config.LoadEffective(home)); err != nil {
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
	if err := writeWrappers(home, absSelf, infos); err != nil {
		return res, err
	}
	if err := ensurePath(home); err != nil {
		return res, err
	}
	removeLeftoverCommands(home)
	res.Status = "ok"
	return res, nil
}

// Uninstall 关闭自动切换并移除包装（配置文件可保留）。
// runtimeNames 为空时关闭总开关并移除全部包装；否则只关闭并移除指定 runtime。
func Uninstall(home string, dryRun bool, runtimeNames []string) (Result, error) {
	res := Result{
		UserConfig: paths.UserConfigFile(home),
		Register:   paths.RegisterFile(home),
		WrapperBin: paths.WrapperBinDir(home),
		Enabled:    false,
	}
	infos, err := selectedRuntimes(runtimeNames)
	if err != nil {
		return res, err
	}
	if dryRun {
		res.Status = "dry_run"
		return res, nil
	}
	cfg := config.Load(home)
	if len(runtimeNames) == 0 {
		cfg.Enabled = false
	} else {
		for _, info := range infos {
			rt := cfg.Runtimes[info.Name]
			rt.Enabled = false
			cfg.Runtimes[info.Name] = rt
		}
		res.Enabled = cfg.Enabled
	}
	if err := config.Save(home, cfg); err != nil {
		return res, err
	}
	dir := paths.WrapperBinDir(home)
	for _, name := range wrapperNames(infos) {
		_ = os.Remove(filepath.Join(dir, name))
	}
	legacyDir := filepath.Join(paths.LeftoverDataDir(home), "bin")
	for _, name := range wrapperNames(infos) {
		_ = os.Remove(filepath.Join(legacyDir, name))
	}
	if len(runtimeNames) == 0 || !wrappersRemaining(home) {
		_ = removePath(home)
	}
	removeLeftoverCommands(home)
	st := State{Enabled: cfg.Enabled}
	if prev, err := readState(home); err == nil {
		st.SelfBinary = prev.SelfBinary
	}
	_ = writeState(home, st)
	res.Status = "ok"
	return res, nil
}

func selectedRuntimes(names []string) ([]aamruntime.Info, error) {
	if len(names) == 0 {
		return aamruntime.All, nil
	}
	for _, name := range names {
		if !config.IsValidRuntime(name) {
			return nil, fmt.Errorf("非法 runtime %q（允许：%s）", name, strings.Join(config.ValidRuntimes, ", "))
		}
	}
	return aamruntime.Filter(names), nil
}

func removeLeftoverCommands(home string) {
	dir := paths.LocalBinDir(home)
	name := paths.LeftoverCommandName()
	_ = os.Remove(filepath.Join(dir, name))
	_ = os.Remove(filepath.Join(dir, name+".exe"))
}

func wrappersRemaining(home string) bool {
	dir := paths.WrapperBinDir(home)
	for _, name := range wrapperNames(nil) {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func wrapperNames(runtimes []aamruntime.Info) []string {
	if len(runtimes) == 0 {
		runtimes = aamruntime.All
	}
	var names []string
	for _, rt := range runtimes {
		for _, w := range rt.WrapperNames {
			if stdruntime.GOOS == "windows" {
				names = append(names, w+".cmd")
			} else {
				names = append(names, w)
			}
		}
	}
	return names
}

func writeWrappers(home, selfBinary string, runtimes []aamruntime.Info) error {
	if len(runtimes) == 0 {
		runtimes = aamruntime.All
	}
	dir := paths.WrapperBinDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, rt := range runtimes {
		for _, name := range rt.WrapperNames {
			if err := writeWrapper(dir, selfBinary, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeWrapper(dir, selfBinary, name string) error {
	if stdruntime.GOOS == "windows" {
		body := fmt.Sprintf("@echo off\r\nREM 由 agent-auto-model install 生成\r\n%q exec --invoked-as %s -- %%*\r\n", selfBinary, name)
		return os.WriteFile(filepath.Join(dir, name+".cmd"), []byte(body), 0o755)
	}
	body := fmt.Sprintf(`#!/bin/sh
# 由 agent-auto-model install 生成：注入 Mode→模型挂钩后转调官方 CLI。
exec %q exec --invoked-as %q -- "$@"
`, selfBinary, name)
	return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755)
}

func pathExportLine(home string) string {
	if stdruntime.GOOS == "windows" {
		return paths.WrapperBinDir(home)
	}
	return fmt.Sprintf(`export PATH=%q:$PATH`, paths.WrapperBinDir(home))
}

func ensurePath(home string) error {
	if stdruntime.GOOS == "windows" {
		return ensureWindowsUserPath(paths.WrapperBinDir(home))
	}
	return ensureUnixPathSnippet(home)
}

func removePath(home string) error {
	if stdruntime.GOOS == "windows" {
		return removeWindowsUserPath(paths.WrapperBinDir(home))
	}
	return removeUnixPathSnippets(home)
}

const pathMarker = "# agent-auto-model PATH"
const legacyPathMarker = "# cursor-mode-model PATH"

func ensureUnixPathSnippet(home string) error {
	snippetDir := filepath.Join(paths.DataDir(home), "shell")
	if err := os.MkdirAll(snippetDir, 0o755); err != nil {
		return err
	}
	snippet := filepath.Join(snippetDir, "path.sh")
	body := "# agent-auto-model：让包装后的 CLI 优先于官方入口\n" + pathExportLine(home) + "\n"
	if err := os.WriteFile(snippet, []byte(body), 0o644); err != nil {
		return err
	}
	line := fmt.Sprintf(`[ -f %q ] && . %q`, snippet, snippet)
	// 含 .profile：Ubuntu login shell 会在 source .bashrc 之后再把 ~/.local/bin
	// 顶回 PATH 最前；必须在 .profile 末尾再 source 一次，包装器才能生效。
	for _, rcName := range []string{".zshrc", ".bashrc", ".zprofile", ".profile"} {
		rc := filepath.Join(home, rcName)
		data, err := os.ReadFile(rc)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		text := stripPathBlock(string(data))
		if strings.Contains(text, pathMarker) {
			if text != string(data) {
				if err := os.WriteFile(rc, []byte(text), 0o644); err != nil {
					return err
				}
			}
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
	for _, rcName := range []string{".zshrc", ".bashrc", ".zprofile", ".profile"} {
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
		if strings.TrimSpace(line) == pathMarker || strings.TrimSpace(line) == legacyPathMarker {
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			if (strings.Contains(line, "agent-auto-model") || strings.Contains(line, "cursor-mode-model")) && strings.Contains(line, "path.sh") {
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
$dir = $env:AAM_PATH_DIR
$remove = $env:AAM_PATH_REMOVE -eq '1'
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
		"AAM_PATH_DIR="+dir,
		fmt.Sprintf("AAM_PATH_REMOVE=%d", boolTo01(remove)),
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

// LoadState 读取安装状态，供诊断和自更新使用。
func LoadState(home string) (State, error) {
	return readState(home)
}
