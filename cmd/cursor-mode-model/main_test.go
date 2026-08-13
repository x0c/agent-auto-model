package main

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
	t.Setenv("CURSOR_MODE_MODEL_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	code := captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "set", "plan", "claude-opus-5-thinking-high", "--json"})
	})
	if code != 0 {
		t.Fatalf("set exit=%d", code)
	}

	out := captureStdout(t, func() {
		if c := run([]string{"cursor-mode-model", "config", "show", "--json"}); c != 0 {
			t.Fatalf("show exit=%d", c)
		}
	})
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Models map[string]string `json:"models"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if !payload.OK || payload.Data.Models["plan"] != "claude-opus-5-thinking-high" {
		t.Fatalf("unexpected: %s", out)
	}

	bad := captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "set", "nope", "x", "--json"})
	})
	if bad == 0 {
		t.Fatal("非法 mode 应非零退出")
	}

	code = captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "set-many", "default=cursor-grok-*-high", "search=cursor-grok-*-high", "--json"})
	})
	if code != 0 {
		t.Fatalf("set-many exit=%d", code)
	}
	code = captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "set-strict", "true", "--json"})
	})
	if code != 0 {
		t.Fatalf("set-strict exit=%d", code)
	}
	code = captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "disable", "--json"})
	})
	if code != 0 {
		t.Fatalf("disable exit=%d", code)
	}
	code = captureExit(t, func() int {
		return run([]string{"cursor-mode-model", "config", "reset", "--json"})
	})
	if code != 0 {
		t.Fatalf("reset exit=%d", code)
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
	// 吞掉 stdout/stderr 噪声
	oldOut, oldErr := os.Stdout, os.Stderr
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout, os.Stderr = devNull, devNull
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = devNull.Close()
	}()
	return fn()
}

func TestUsageMentionsConfig(t *testing.T) {
	errOut := captureStderr(t, func() {
		_ = usage()
	})
	if !strings.Contains(errOut, "config show") {
		t.Fatalf("usage 缺少 config: %s", errOut)
	}
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
