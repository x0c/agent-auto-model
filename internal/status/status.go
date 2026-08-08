// Package status 汇总本机安装与锚点健康度。
package status

import (
	"os"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/agentbin"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/anchors"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/config"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/wrap"
)

// Payload status 命令输出。
type Payload struct {
	EnabledConfig bool              `json:"enabled_config"`
	EnabledEnv    bool              `json:"enabled_env"`
	Active        bool              `json:"active"`
	UserConfig    string            `json:"user_config"`
	Register      string            `json:"register"`
	RegisterOK    bool              `json:"register_present"`
	WrapperBin    string            `json:"wrapper_bin"`
	PathAgent     string            `json:"path_agent"`
	RealAgent     string            `json:"real_agent"`
	Models        map[string]string `json:"models"`
	Anchors       anchors.Result    `json:"anchors"`
	Hint          string            `json:"hint,omitempty"`
}

// Collect 收集状态。
func Collect(home string) Payload {
	cfg := config.Load(home)
	reg := paths.RegisterFile(home)
	_, regErr := os.Stat(reg)
	real, _ := agentbin.Find(home)
	p := Payload{
		EnabledConfig: cfg.Enabled,
		EnabledEnv:    !config.GloballyDisabled(),
		Active:        cfg.Enabled && !config.GloballyDisabled(),
		UserConfig:    paths.UserConfigFile(home),
		Register:      reg,
		RegisterOK:    regErr == nil,
		WrapperBin:    paths.WrapperBinDir(home),
		PathAgent:     wrap.LookPath("agent"),
		RealAgent:     real,
		Models:        cfg.Models,
		Anchors:       anchors.Check(home),
	}
	if !p.Anchors.OK {
		p.Hint = "当前 Cursor Agent 打包锚点未完全命中；自动切换会故障开放。升级 Agent 后请再跑 status。"
	}
	return p
}
