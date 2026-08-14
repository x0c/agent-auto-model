// Package status 汇总本机安装与各 runtime 健康度。
package status

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/x0c/agent-auto-model/internal/agentbin"
	"github.com/x0c/agent-auto-model/internal/anchors"
	"github.com/x0c/agent-auto-model/internal/autoupdate"
	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/recommended"
	aamruntime "github.com/x0c/agent-auto-model/internal/runtime"
	"github.com/x0c/agent-auto-model/internal/runtime/codex"
	"github.com/x0c/agent-auto-model/internal/wrap"
)

// Decision 审计日志中的一条决策摘要。
type Decision struct {
	T        int64  `json:"t,omitempty"`
	Ev       string `json:"ev,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Method   string `json:"method,omitempty"`
}

// RuntimePayload 单个 runtime 的状态。
type RuntimePayload struct {
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	WrapperEffective bool              `json:"wrapper_effective"`
	Wrappers         map[string]string `json:"wrappers"`
	RealBinary       string            `json:"real_binary,omitempty"`
	Models           map[string]string `json:"models"`
	Anchors          *anchors.Result   `json:"anchors,omitempty"`
	RecentDecisions  []Decision        `json:"recent_decisions,omitempty"`
	Hint             string            `json:"hint,omitempty"`
}

// Payload status 命令输出。
type Payload struct {
	EnabledConfig   bool                      `json:"enabled_config"`
	EnabledEnv      bool                      `json:"enabled_env"`
	Strict          bool                      `json:"strict"`
	Active          bool                      `json:"active"`
	UserConfig      string                    `json:"user_config"`
	Register        string                    `json:"register"`
	RegisterOK      bool                      `json:"register_present"`
	WrapperBin      string                    `json:"wrapper_bin"`
	Models          map[string]string         `json:"models"`
	Runtimes        []RuntimePayload          `json:"runtimes"`
	AutoUpdate      autoupdate.RuntimeStatus  `json:"auto_update"`
	ModelsSource    string                    `json:"models_source"`
	ModelsSourceTag string                    `json:"models_source_tag"`
	Recommended     recommended.RuntimeStatus `json:"recommended"`
	Hint            string                    `json:"hint,omitempty"`

	// 兼容旧字段（Cursor）。
	WrapperEffective bool           `json:"wrapper_effective"`
	PathAgent        string         `json:"path_agent"`
	RealAgent        string         `json:"real_agent"`
	Anchors          anchors.Result `json:"anchors"`
	RecentDecisions  []Decision     `json:"recent_decisions,omitempty"`
}

// Collect 收集状态。filter 为空表示全部 runtime。
func Collect(home string, filter []string) Payload {
	cfg := config.LoadEffective(home)
	reg := paths.RegisterFile(home)
	_, regErr := os.Stat(reg)
	infos := aamruntime.Filter(filter)
	runtimes := make([]RuntimePayload, 0, len(infos))
	anyActive := false
	var firstHint string
	for _, info := range infos {
		rp := collectRuntime(home, cfg, info)
		runtimes = append(runtimes, rp)
		if rp.Enabled && rp.WrapperEffective {
			anyActive = true
		}
		if firstHint == "" && rp.Hint != "" {
			firstHint = rp.Hint
		}
	}
	cursor := runtimeByName(runtimes, config.RuntimeCursor)
	p := Payload{
		EnabledConfig:    cfg.Enabled,
		EnabledEnv:       !config.GloballyDisabled(),
		Strict:           cfg.Strict,
		Active:           cfg.Enabled && !config.GloballyDisabled() && anyActive,
		UserConfig:       paths.UserConfigFile(home),
		Register:         reg,
		RegisterOK:       regErr == nil,
		WrapperBin:       paths.WrapperBinDir(home),
		Models:           config.ModelsFor(cfg, config.RuntimeCursor),
		Runtimes:         runtimes,
		AutoUpdate:       autoupdate.LoadRuntimeStatus(home),
		ModelsSource:     cfg.ModelsSource,
		ModelsSourceTag:  config.ModelsSourceTag(cfg.ModelsSource),
		Recommended:      recommended.Status(home),
		WrapperEffective: cursor.WrapperEffective,
		PathAgent:        cursor.Wrappers["agent"],
		RealAgent:        cursor.RealBinary,
		RecentDecisions:  cursor.RecentDecisions,
		Hint:             firstHint,
	}
	if cursor.Anchors != nil {
		p.Anchors = *cursor.Anchors
	}
	if !p.EnabledConfig || !p.EnabledEnv {
		p.Hint = "配置或环境变量已关闭自动切换。"
	} else if cfg.ModelsSource == config.ModelsSourceLocal {
		p.Hint = "模型映射来源是本地自定义，不会跟随仓库推荐配置。"
	}
	return p
}

func collectRuntime(home string, cfg config.Config, info aamruntime.Info) RuntimePayload {
	wrappers := map[string]string{}
	effective := false
	for _, name := range info.WrapperNames {
		p := wrap.LookPath(name)
		wrappers[name] = p
		if wrapperEffective(home, p) {
			effective = true
		}
	}
	rp := RuntimePayload{
		Name:             info.Name,
		Enabled:          config.RuntimeEnabled(cfg, info.Name) && !config.GloballyDisabled(),
		WrapperEffective: effective,
		Wrappers:         wrappers,
		Models:           config.ModelsFor(cfg, info.Name),
	}
	switch info.Name {
	case config.RuntimeCursor:
		real, _ := agentbin.Find(home)
		rp.RealBinary = real
		ar := anchors.Check(home)
		rp.Anchors = &ar
		rp.RecentDecisions = recentDecisions(paths.DecisionsLog(home), 8)
		switch {
		case !effective:
			rp.Hint = "当前 PATH 上的 agent 不是本工具包装器；请执行 install，并确保包装目录在 PATH 最前。"
		case !ar.OK:
			rp.Hint = "当前 Cursor Agent 打包锚点未完全命中；自动切换会故障开放。"
		}
	case config.RuntimeCodex:
		real, _ := codex.FindReal(home)
		rp.RealBinary = real
		rp.RecentDecisions = recentDecisions(paths.CodexDecisionsLog(home), 8)
		if !effective {
			rp.Hint = "当前 PATH 上的 codex 不是本工具包装器；请执行 install，并确保包装目录在 PATH 最前。"
		} else if real == "" {
			rp.Hint = "未找到官方 Codex CLI；请先安装 @openai/codex。"
		}
	}
	return rp
}

func runtimeByName(list []RuntimePayload, name string) RuntimePayload {
	for _, r := range list {
		if r.Name == name {
			return r
		}
	}
	return RuntimePayload{Wrappers: map[string]string{}}
}

func wrapperEffective(home, pathAgent string) bool {
	if pathAgent == "" {
		return false
	}
	wrapperDir := paths.WrapperBinDir(home)
	if strings.HasPrefix(pathAgent, wrapperDir+string(os.PathSeparator)) || pathAgent == wrapperDir {
		return true
	}
	legacy := filepath.Join(paths.LeftoverDataDir(home), "bin")
	if strings.HasPrefix(pathAgent, legacy+string(os.PathSeparator)) {
		return true
	}
	data, err := os.ReadFile(pathAgent)
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "agent-auto-model") &&
		(strings.Contains(text, " exec ") || strings.Contains(text, "\"exec\""))
}

func recentDecisions(path string, limit int) []Decision {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) == 0 {
		return nil
	}
	if limit > len(lines) {
		limit = len(lines)
	}
	start := len(lines) - limit
	out := make([]Decision, 0, limit)
	for _, line := range lines[start:] {
		var d Decision
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
