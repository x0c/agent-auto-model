package rewrite

import (
	"encoding/json"
	"strings"
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

func TestInitializeInjectsExperimentalAPI(t *testing.T) {
	st := NewState(nil, false)
	out, _ := RewriteIncoming([]byte(`{"id":"initialize","method":"initialize","params":{"clientInfo":{"name":"codex-tui","version":"0.145.0"}}}`), st)
	if !strings.Contains(string(out), `"experimentalApi":true`) {
		t.Fatalf("out=%s", out)
	}
}

func TestObserveModelListAndGlob(t *testing.T) {
	st := NewState(map[string]string{"default": "gpt-5.6-*:medium"}, false)
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
