package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCorralClaimAcceptsOnlyThreadStartedWithPersistentThread(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claim.json")
	t.Setenv("CORRAL_CODEX_CLAIM_PATH", path)
	writeCorralClaim([]byte(`{"method":"thread/started","params":{"thread":{"id":"01a04865-03cc-71e2-9666-f2c849dcbe6f","path":"/tmp/rollout.jsonl"}}}`))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var claim map[string]any
	if err := json.Unmarshal(raw, &claim); err != nil {
		t.Fatal(err)
	}
	if claim["thread_id"] != "01a04865-03cc-71e2-9666-f2c849dcbe6f" || claim["rollout_path"] != "/tmp/rollout.jsonl" {
		t.Fatalf("claim=%v", claim)
	}
	if _, ok := claim["pid"].(float64); !ok {
		t.Fatalf("claim=%v", claim)
	}
}

func TestWriteCorralClaimRejectsIncompleteOrOtherEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claim.json")
	t.Setenv("CORRAL_CODEX_CLAIM_PATH", path)
	writeCorralClaim([]byte(`{"method":"thread/started","params":{"thread":{"id":"id"}}}`))
	writeCorralClaim([]byte(`{"method":"thread/status/changed","params":{"threadId":"id"}}`))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected claim: %v", err)
	}
}
