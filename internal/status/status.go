// Package status 汇总本机安装与锚点健康度。
package status

import (
	"os"
	"strings"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/agentbin"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/anchors"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/config"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
	"forgejo.caozc.top/Max/cursor-mode-model/internal/wrap"
)

// Payload status 命令输出。
type Payload struct {
	EnabledConfig     bool              `json:"enabled_config"`
	EnabledEnv        bool              `json:"enabled_env"`
	WrapperEffective  bool              `json:"wrapper_effective"`
	Active            bool              `json:"active"`
	UserConfig        string            `json:"user_config"`
	Register          string            `json:"register"`
	RegisterOK        bool              `json:"register_present"`
	WrapperBin        string            `json:"wrapper_bin"`
	PathAgent         string            `json:"path_agent"`
	RealAgent         string            `json:"real_agent"`
	Models            map[string]string `json:"models"`
	Anchors           anchors.Result    `json:"anchors"`
	Hint              string            `json:"hint,omitempty"`
}

// Collect 收集状态。
func Collect(home string) Payload {
	cfg := config.Load(home)
	reg := paths.RegisterFile(home)
	_, regErr := os.Stat(reg)
	real, _ := agentbin.Find(home)
	pathAgent := wrap.LookPath("agent")
	wrapperOK := wrapperEffective(home, pathAgent)
	p := Payload{
		EnabledConfig:    cfg.Enabled,
		EnabledEnv:       !config.GloballyDisabled(),
		WrapperEffective: wrapperOK,
		Active:           cfg.Enabled && !config.GloballyDisabled() && wrapperOK && regErr == nil,
		UserConfig:       paths.UserConfigFile(home),
		Register:         reg,
		RegisterOK:       regErr == nil,
		WrapperBin:       paths.WrapperBinDir(home),
		PathAgent:        pathAgent,
		RealAgent:        real,
		Models:           cfg.Models,
		Anchors:          anchors.Check(home),
	}
	switch {
	case !p.WrapperEffective:
		p.Hint = "当前 PATH 上的 agent 不是本工具包装器；请执行 install，并确保 ~/.local/share/cursor-mode-model/bin 在 PATH 最前（新开终端或 source shell rc）。配置开启不等于已生效。"
	case !p.Anchors.OK:
		p.Hint = "当前 Cursor Agent 打包锚点未完全命中；自动切换会故障开放。升级 Agent 后请再跑 status。"
	case !p.RegisterOK:
		p.Hint = "预加载脚本缺失；请重新执行 install。"
	case !p.EnabledConfig || !p.EnabledEnv:
		p.Hint = "配置或环境变量已关闭自动切换。"
	}
	return p
}

func wrapperEffective(home, pathAgent string) bool {
	if pathAgent == "" {
		return false
	}
	wrapperDir := paths.WrapperBinDir(home)
	if strings.HasPrefix(pathAgent, wrapperDir+string(os.PathSeparator)) || pathAgent == wrapperDir {
		return true
	}
	data, err := os.ReadFile(pathAgent)
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "cursor-mode-model") &&
		(strings.Contains(text, " exec ") || strings.Contains(text, "\"exec\""))
}
