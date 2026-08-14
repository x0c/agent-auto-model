package spec

import (
	"regexp"
	"strings"
)

// Spec Codex 模型规格：model[:effort]。
type Spec struct {
	Model  string
	Effort string
}

var effortRe = regexp.MustCompile(`(?i)^(none|minimal|low|medium|high|xhigh|max|ultra)$`)

// Parse 解析 "gpt-5.6-sol:high" / "gpt-5.6-terra"。
func Parse(raw string) Spec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}
	}
	model, effort, ok := strings.Cut(raw, ":")
	if !ok {
		return Spec{Model: raw}
	}
	model = strings.TrimSpace(model)
	effort = strings.TrimSpace(effort)
	if model == "" {
		return Spec{}
	}
	return Spec{Model: model, Effort: effort}
}

// String 还原为 model[:effort]。
func (s Spec) String() string {
	if s.Model == "" {
		return ""
	}
	if s.Effort == "" {
		return s.Model
	}
	return s.Model + ":" + s.Effort
}

// Empty 是否未设置。
func (s Spec) Empty() bool {
	return s.Model == ""
}

// ValidEffort 档位是否在已知集合内；空档位视为合法（沿用模型默认）。
func ValidEffort(effort string) bool {
	if effort == "" {
		return true
	}
	return effortRe.MatchString(effort)
}

// IsGlob 规格的模型段是否含通配符。
func IsGlob(spec string) bool {
	s := Parse(spec)
	return strings.ContainsAny(s.Model, "*?")
}
