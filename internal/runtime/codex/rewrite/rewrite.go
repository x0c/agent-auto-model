// Package rewrite 改写 Codex app-server JSON-RPC，按 collaborationMode 强制模型。
package rewrite

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/runtime/codex/spec"
)

const (
	MethodInitialize          = "initialize"
	MethodThreadStart         = "thread/start"
	MethodThreadResume        = "thread/resume"
	MethodThreadSettingsUpdate = "thread/settings/update"
	MethodTurnStart            = "turn/start"
)

// Decision 一条改写/跳过审计。
type Decision struct {
	Ev       string `json:"ev"`
	Method   string `json:"method,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// State 会话改写状态。
type State struct {
	Locked    bool
	LastMode  string
	Plan      spec.Spec
	Default   spec.Spec
	Available []string
}

// NewState 从配置映射构造。
func NewState(models map[string]string, locked bool) *State {
	st := &State{Locked: locked}
	if models != nil {
		st.Plan = spec.Parse(models["plan"])
		st.Default = spec.Parse(models["default"])
	}
	return st
}

// SpecFor 按 mode 取规格。
func (s *State) SpecFor(mode string) spec.Spec {
	if strings.EqualFold(mode, "plan") {
		if !s.Plan.Empty() {
			return s.Plan
		}
	}
	return s.Default
}

// Message JSON-RPC 对象（map 以便保留未知字段）。
type Message map[string]any

func (m Message) Method() string {
	v, _ := m["method"].(string)
	return v
}

func (m Message) Params() map[string]any {
	switch p := m["params"].(type) {
	case map[string]any:
		return p
	default:
		return nil
	}
}

// RewriteIncoming 改写 TUI→server 的请求。返回改写后的消息与决策（可能为空）。
func RewriteIncoming(raw []byte, st *State) ([]byte, *Decision) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return raw, nil
	}
	method := msg.Method()
	if method == MethodInitialize {
		ensureExperimentalAPI(msg)
		out, err := json.Marshal(msg)
		if err != nil {
			return raw, nil
		}
		return out, nil
	}
	if st == nil || st.Locked {
		if st != nil && st.Locked && isModelMethod(method) {
			return raw, &Decision{Ev: "skip", Method: method, Reason: "locked"}
		}
		return raw, nil
	}
	if !isModelMethod(method) {
		return raw, nil
	}
	params := msg.Params()
	if params == nil {
		params = map[string]any{}
		msg["params"] = params
	}
	mode := collabMode(params)
	if shouldLock(method, params, mode, st) {
		st.Locked = true
		return raw, &Decision{Ev: "lock", Method: method, Mode: mode, Actual: stringField(params, "model"), Reason: "explicit_model"}
	}
	if mode != "" {
		st.LastMode = mode
	}
	targetMode := mode
	if targetMode == "" {
		if method == MethodThreadStart || method == MethodThreadResume {
			targetMode = "default"
		} else if st.LastMode != "" {
			targetMode = st.LastMode
		} else {
			targetMode = "default"
		}
	}
	want := st.SpecFor(targetMode)
	if want.Empty() {
		return raw, &Decision{Ev: "skip", Method: method, Mode: targetMode, Reason: "no_mapping"}
	}
	resolved := expandSpec(want, st.Available)
	before := spec.Spec{Model: stringField(params, "model"), Effort: stringField(params, "effort")}
	applySpec(params, resolved)
	msg["params"] = params
	out, err := json.Marshal(msg)
	if err != nil {
		return raw, nil
	}
	return out, &Decision{
		Ev:       "corrected",
		Method:   method,
		Mode:     targetMode,
		Expected: resolved.String(),
		Actual:   before.String(),
	}
}

// ObserveOutgoing 从 server→TUI 响应里缓存 model/list。
func ObserveOutgoing(raw []byte, st *State) {
	if st == nil {
		return
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		return
	}
	data, ok := result["data"].([]any)
	if !ok {
		return
	}
	ids := make([]string, 0, len(data))
	for _, item := range data {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id := stringField(obj, "id"); id != "" {
			ids = append(ids, id)
		} else if id := stringField(obj, "model"); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		st.Available = ids
	}
}

func isModelMethod(method string) bool {
	switch method {
	case MethodThreadStart, MethodThreadResume, MethodThreadSettingsUpdate, MethodTurnStart:
		return true
	default:
		return false
	}
}

func ensureExperimentalAPI(msg Message) {
	params := msg.Params()
	if params == nil {
		params = map[string]any{}
		msg["params"] = params
	}
	caps, _ := params["capabilities"].(map[string]any)
	if caps == nil {
		caps = map[string]any{}
	}
	caps["experimentalApi"] = true
	params["capabilities"] = caps
}

func collabMode(params map[string]any) string {
	cm, ok := params["collaborationMode"].(map[string]any)
	if !ok {
		return ""
	}
	mode, _ := cm["mode"].(string)
	return strings.TrimSpace(mode)
}

func shouldLock(method string, params map[string]any, mode string, st *State) bool {
	if method != MethodThreadSettingsUpdate && method != MethodTurnStart {
		return false
	}
	model := stringField(params, "model")
	if model == "" {
		return false
	}
	if mode != "" && mode != st.LastMode && st.LastMode != "" {
		// 模式正在切换且顺带带了 model：按模式映射，不上锁。
		return false
	}
	want := st.SpecFor(modeOr(mode, st.LastMode, "default"))
	resolved := expandSpec(want, st.Available)
	if resolved.Model != "" && model == resolved.Model {
		return false
	}
	if want.Model != "" && (model == want.Model || matchGlob(want.Model, model)) {
		return false
	}
	return true
}

func modeOr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func applySpec(params map[string]any, s spec.Spec) {
	if s.Model != "" {
		params["model"] = s.Model
	}
	if s.Effort != "" {
		params["effort"] = s.Effort
	}
	cm, ok := params["collaborationMode"].(map[string]any)
	if !ok {
		return
	}
	settings, _ := cm["settings"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
	}
	if s.Model != "" {
		settings["model"] = s.Model
	}
	if s.Effort != "" {
		settings["reasoning_effort"] = s.Effort
	}
	cm["settings"] = settings
	params["collaborationMode"] = cm
}

func expandSpec(s spec.Spec, available []string) spec.Spec {
	if s.Model == "" || !strings.ContainsAny(s.Model, "*?") {
		return s
	}
	picked := pickLatestMatching(s.Model, available)
	if picked == "" {
		return s
	}
	s.Model = picked
	return s
}

func matchGlob(pattern, value string) bool {
	re := globToRegexp(pattern)
	return re.MatchString(value)
}

func pickLatestMatching(pattern string, candidates []string) string {
	re := globToRegexp(pattern)
	var matched []string
	for _, id := range candidates {
		if re.MatchString(id) {
			matched = append(matched, id)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	sort.Slice(matched, func(i, j int) bool {
		return compareModelCandidates(matched[i], matched[j]) < 0
	})
	return matched[len(matched)-1]
}

func globToRegexp(spec string) *regexp.Regexp {
	var b strings.Builder
	b.WriteByte('^')
	for _, ch := range spec {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String())
}

func compareModelCandidates(a, b string) int {
	va := versionKey(a)
	vb := versionKey(b)
	n := len(va)
	if len(vb) > n {
		n = len(vb)
	}
	for i := 0; i < n; i++ {
		x, y := 0, 0
		if i < len(va) {
			x = va[i]
		}
		if i < len(vb) {
			y = vb[i]
		}
		if x != y {
			return x - y
		}
	}
	return strings.Compare(a, b)
}

func versionKey(id string) []int {
	re := regexp.MustCompile(`(\d+)(?:\.(\d+))?`)
	var nums []int
	for _, m := range re.FindAllStringSubmatch(id, -1) {
		n, _ := strconv.Atoi(m[1])
		nums = append(nums, n)
		if m[2] != "" {
			n2, _ := strconv.Atoi(m[2])
			nums = append(nums, n2)
		} else {
			nums = append(nums, 0)
		}
	}
	return nums
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	default:
		return ""
	}
}
