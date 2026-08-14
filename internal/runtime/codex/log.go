package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/runtime/codex/rewrite"
)

func logDecision(home string, d *rewrite.Decision) {
	if d == nil {
		return
	}
	type row struct {
		T int64 `json:"t"`
		rewrite.Decision
	}
	payload, err := json.Marshal(row{T: time.Now().UnixMilli(), Decision: *d})
	if err != nil {
		return
	}
	path := paths.CodexDecisionsLog(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(payload, '\n'))
	_ = f.Close()
}
