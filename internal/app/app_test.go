package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_AUTO_MODEL_HOME", home)
	t.Setenv("CURSOR_MODE_MODEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK", "1")

	code := captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "set", "plan", "claude-opus-5-thinking-high", "--json"})
	})
	if code != 0 {
		t.Fatalf("set exit=%d", code)
	}

	out := captureStdout(t, func() {
		if c := Run([]string{"agent-auto-model", "config", "show", "--json"}); c != 0 {
			t.Fatalf("show exit=%d", c)
		}
	})
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Models   map[string]string `json:"models"`
			Runtimes map[string]struct {
				Models map[string]string `json:"models"`
			} `json:"runtimes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if !payload.OK || payload.Data.Models["plan"] != "claude-opus-5-thinking-high" {
		t.Fatalf("unexpected: %s", out)
	}

	bad := captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "set", "nope", "x", "--json"})
	})
	if bad == 0 {
		t.Fatal("非法 mode 应非零退出")
	}

	code = captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "set", "codex.plan", "gpt-5.6-sol:high", "--json"})
	})
	if code != 0 {
		t.Fatalf("codex set exit=%d", code)
	}
	code = captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "set-many", "default=cursor-grok-*-high", "codex.default=gpt-5.6-terra:medium", "--json"})
	})
	if code != 0 {
		t.Fatalf("set-many exit=%d", code)
	}
	code = captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "set-strict", "true", "--json"})
	})
	if code != 0 {
		t.Fatalf("set-strict exit=%d", code)
	}
	code = captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "disable", "--json"})
	})
	if code != 0 {
		t.Fatalf("disable exit=%d", code)
	}
	code = captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "reset", "--json"})
	})
	if code != 0 {
		t.Fatalf("reset exit=%d", code)
	}
}

func TestUsageMentionsConfig(t *testing.T) {
	errOut := captureStderr(t, func() {
		_ = usage()
	})
	if !strings.Contains(errOut, "config show") {
		t.Fatalf("usage 缺少 config: %s", errOut)
	}
	if !strings.Contains(errOut, "codex.plan") {
		t.Fatalf("usage 缺少 codex.plan: %s", errOut)
	}
}

func TestStatusRuntimeFilterAndDisable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_AUTO_MODEL_HOME", home)
	t.Setenv("CURSOR_MODE_MODEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK", "1")

	code := captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "config", "disable", "--runtime", "codex", "--json"})
	})
	if code != 0 {
		t.Fatalf("disable --runtime codex exit=%d", code)
	}
	out := captureStdout(t, func() {
		if c := Run([]string{"agent-auto-model", "status", "--runtime", "codex", "--json"}); c != 0 {
			t.Fatalf("status exit=%d", c)
		}
	})
	if !strings.Contains(out, `"name":"codex"`) {
		t.Fatalf("status 应含 codex: %s", out)
	}
	if strings.Contains(out, `"name":"cursor"`) {
		t.Fatalf("过滤后不应含 cursor: %s", out)
	}
	bad := captureExit(t, func() int {
		return Run([]string{"agent-auto-model", "status", "--runtime", "nope", "--json"})
	})
	if bad == 0 {
		t.Fatal("非法 runtime 应非零退出")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func captureExit(t *testing.T, fn func() int) int {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout, os.Stderr = devNull, devNull
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = devNull.Close()
	}()
	return fn()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
