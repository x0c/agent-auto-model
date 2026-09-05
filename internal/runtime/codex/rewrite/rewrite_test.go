package rewrite

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestRewritePlanSettingsUpdate(t *testing.T) {
	st := NewState(map[string]string{
		"plan":    "gpt-5.6-sol:high",
		"default": "gpt-5.6-terra:medium",
	}, false)
	in := `{"id":6,"method":"thread/settings/update","params":{"threadId":"t1","model":null,"effort":null,"collaborationMode":{"mode":"plan","settings":{"model":null}}}}`
	out, d := RewriteIncoming([]byte(in), st)
	if d == nil || d.Ev != "corrected" || d.Mode != "plan" {
		t.Fatalf("decision=%#v", d)
	}
	var msg map[string]any
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatal(err)
	}
	params := msg["params"].(map[string]any)
	if params["model"] != "gpt-5.6-sol" || params["effort"] != "high" {
		t.Fatalf("params=%#v", params)
	}
	cm := params["collaborationMode"].(map[string]any)
	settings := cm["settings"].(map[string]any)
	if settings["model"] != "gpt-5.6-sol" || settings["reasoning_effort"] != "high" {
		t.Fatalf("settings=%#v", settings)
	}
}

func TestRewriteDefaultAfterPlan(t *testing.T) {
	st := NewState(map[string]string{
		"plan":    "gpt-5.6-sol:high",
		"default": "gpt-5.6-terra:medium",
	}, false)
	_, _ = RewriteIncoming([]byte(`{"method":"thread/settings/update","params":{"collaborationMode":{"mode":"plan"}}}`), st)
	out, d := RewriteIncoming([]byte(`{"method":"thread/settings/update","params":{"collaborationMode":{"mode":"default"}}}`), st)
	if d == nil || d.Mode != "default" {
		t.Fatalf("decision=%#v", d)
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-terra"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestThreadStartAppliesDefault(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-5.6-terra:medium"}, false)
	out, d := RewriteIncoming([]byte(`{"method":"thread/start","params":{"model":"gpt-5.6-terra","cwd":"/tmp"}}`), st)
	if d == nil || d.Ev != "corrected" {
		t.Fatalf("decision=%#v", d)
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-terra"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestExplicitModelLocks(t *testing.T) {
	st := NewState(map[string]string{
		"plan":    "gpt-5.6-sol:high",
		"default": "gpt-5.6-terra:medium",
	}, false)
	st.LastMode = "default"
	in := `{"method":"thread/settings/update","params":{"model":"gpt-5.6-luna","collaborationMode":{"mode":"default"}}}`
	_, d := RewriteIncoming([]byte(in), st)
	if d == nil || d.Ev != "lock" || !st.Locked {
		t.Fatalf("decision=%#v locked=%v", d, st.Locked)
	}
	_, d = RewriteIncoming([]byte(`{"method":"thread/settings/update","params":{"collaborationMode":{"mode":"plan"}}}`), st)
	if d == nil || d.Reason != "locked" {
		t.Fatalf("after lock: %#v", d)
	}
}

func TestMappedModelDoesNotLock(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-5.6-terra:medium"}, false)
	st.LastMode = "default"
	_, d := RewriteIncoming([]byte(`{"method":"thread/settings/update","params":{"model":"gpt-5.6-terra","collaborationMode":{"mode":"default"}}}`), st)
	if st.Locked {
		t.Fatalf("should not lock: %#v", d)
	}
}

// requestModelList 模拟 TUI 发出 model/list 请求，让后续同 id 响应可被观察。
func requestModelList(t *testing.T, st *State, id string) {
	t.Helper()
	_, _ = RewriteIncoming([]byte(`{"id":`+id+`,"method":"model/list","params":{}}`), st)
}

func TestInitializeInjectsExperimentalAPI(t *testing.T) {
	st := NewState(nil, false)
	out, _ := RewriteIncoming([]byte(`{"id":"initialize","method":"initialize","params":{"clientInfo":{"name":"codex-tui","version":"0.145.0"}}}`), st)
	if !strings.Contains(string(out), `"experimentalApi":true`) {
		t.Fatalf("out=%s", out)
	}
}

func TestObserveModelListAndGlob(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-5.6-*:medium"}, false)
	requestModelList(t, st, "3")
	ObserveOutgoing([]byte(`{"id":3,"result":{"data":[{"id":"gpt-5.6-luna"},{"id":"gpt-5.6-terra"},{"id":"gpt-5.6-sol"}]}}`), st)
	out, d := RewriteIncoming([]byte(`{"method":"thread/start","params":{}}`), st)
	if d == nil || d.Expected != "gpt-5.6-terra:medium" && d.Expected != "gpt-5.6-sol:medium" {
		// latest by version then name; all 5.6, name compare: luna < sol < terra so terra last
		if d == nil || !strings.HasPrefix(d.Expected, "gpt-5.6-") {
			t.Fatalf("decision=%#v out=%s", d, out)
		}
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-terra"`) {
		t.Fatalf("expected terra latest, out=%s decision=%#v", out, d)
	}
}

func TestObserveModelListFamilyGlob(t *testing.T) {
	st := NewState(map[string]string{
		"plan":    "gpt-*-sol:high",
		"default": "gpt-*-terra:medium",
	}, false)
	requestModelList(t, st, "3")
	ObserveOutgoing([]byte(`{"id":3,"result":{"data":[{"id":"gpt-5.4-sol"},{"id":"gpt-5.6-sol"},{"id":"gpt-5.4-terra"},{"id":"gpt-5.6-terra"}]}}`), st)
	out, d := RewriteIncoming([]byte(`{"method":"thread/start","params":{"collaborationMode":{"mode":"plan"}}}`), st)
	if d == nil || d.Mode != "plan" {
		t.Fatalf("decision=%#v out=%s", d, out)
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-sol"`) {
		t.Fatalf("expected latest sol, out=%s decision=%#v", out, d)
	}
	out, d = RewriteIncoming([]byte(`{"method":"thread/start","params":{"collaborationMode":{"mode":"default"}}}`), st)
	if d == nil || d.Mode != "default" {
		t.Fatalf("decision=%#v out=%s", d, out)
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-terra"`) {
		t.Fatalf("expected latest terra, out=%s decision=%#v", out, d)
	}
}

func TestTurnStartPlan(t *testing.T) {
	st := NewState(map[string]string{"plan": "gpt-5.6-sol:high", "default": "gpt-5.6-terra:medium"}, false)
	out, d := RewriteIncoming([]byte(`{"method":"turn/start","params":{"threadId":"t","input":[],"collaborationMode":{"mode":"plan","settings":{"model":"gpt-5.6-terra"}}}}`), st)
	if d == nil || d.Mode != "plan" {
		t.Fatalf("%#v", d)
	}
	if !strings.Contains(string(out), `"model":"gpt-5.6-sol"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestObserveIgnoresNonModelListResponses(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-*-terra:medium"}, false)
	requestModelList(t, st, "3")
	ObserveOutgoing([]byte(`{"id":3,"result":{"data":[{"id":"gpt-5.6-terra"}]}}`), st)
	// thread 等其它响应携带 result.data（消息/turn id）不得覆盖模型目录
	ObserveOutgoing([]byte(`{"id":7,"result":{"data":[{"id":"msg-1"},{"id":"turn-2"}]}}`), st)
	out, d := RewriteIncoming([]byte(`{"method":"thread/start","params":{}}`), st)
	if d == nil || d.Ev != "corrected" || d.Expected != "gpt-5.6-terra:medium" {
		t.Fatalf("decision=%#v out=%s", d, out)
	}
}

func TestUnresolvedGlobNotInjected(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-*-terra:medium"}, false)
	// 可用模型目录为空：绝不把通配符写进请求
	out, d := RewriteIncoming([]byte(`{"method":"turn/start","params":{"model":"gpt-5.6-terra","effort":"medium"}}`), st)
	if d == nil || d.Ev != "skip" || d.Reason != "unresolved_glob" {
		t.Fatalf("decision=%#v", d)
	}
	if string(out) != `{"method":"turn/start","params":{"model":"gpt-5.6-terra","effort":"medium"}}` {
		t.Fatalf("请求被改动：%s", out)
	}
}

func TestModelListResponseAfterCatalogLost(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-*-terra:medium"}, false)
	// 未跟踪 id 的 model/list 形状响应也不采纳
	ObserveOutgoing([]byte(`{"id":99,"result":{"data":[{"id":"gpt-5.6-terra"}]}}`), st)
	if len(st.Available) != 0 {
		t.Fatalf("未跟踪的响应不应进入目录：%v", st.Available)
	}
}

// TestStateConcurrentObserveAndRewrite 模拟 proxy 上下行两路并发碰 State。
// 配合 go test -race 捕获无锁时的 map/slice 竞态。
func TestStateConcurrentObserveAndRewrite(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-*-terra:medium"}, false)
	const n = 200
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			id := strconv.Itoa(i%7 + 1)
			_, _ = RewriteIncoming([]byte(`{"id":`+id+`,"method":"model/list","params":{}}`), st)
			ObserveOutgoing([]byte(`{"id":`+id+`,"result":{"data":[{"id":"gpt-5.6-terra"},{"id":"gpt-5.6-sol"}]}}`), st)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_, _ = RewriteIncoming([]byte(`{"method":"thread/start","params":{"collaborationMode":{"mode":"default"}}}`), st)
		}
	}()
	wg.Wait()
}

