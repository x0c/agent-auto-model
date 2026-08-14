package codex

import "testing"

func TestIsPassthrough(t *testing.T) {
	if !isPassthrough([]string{"exec", "do something"}) {
		t.Fatal("exec")
	}
	if !isPassthrough([]string{"--remote", "unix:///tmp/x"}) {
		t.Fatal("remote")
	}
	if !isPassthrough([]string{"--help"}) {
		t.Fatal("help")
	}
	if isPassthrough(nil) {
		t.Fatal("bare TUI should intercept")
	}
	if isPassthrough([]string{"resume", "--last"}) {
		t.Fatal("resume is TUI")
	}
	if isPassthrough([]string{"--search", "fix the bug"}) {
		t.Fatal("TUI flags should intercept")
	}
}

func TestArgvHasExplicitModel(t *testing.T) {
	if !argvHasExplicitModel([]string{"--model", "gpt-5.6-sol"}) {
		t.Fatal("--model")
	}
	if !argvHasExplicitModel([]string{"-m=gpt-5.6-terra"}) {
		t.Fatal("-m=")
	}
	if argvHasExplicitModel([]string{"resume"}) {
		t.Fatal("no model")
	}
}
