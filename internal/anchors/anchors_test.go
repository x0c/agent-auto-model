package anchors

import (
	"testing"
)

func TestDefinitionsFromEmbeddedJSON(t *testing.T) {
	defs := Definitions()
	for _, key := range []string{
		"setCurrentModel",
		"setCurrentModelWithParameters",
		"setModelFromStoredId",
		"getCurrentModel",
		"setMetadata",
		"buildRequestedModel",
	} {
		if defs[key] == "" {
			t.Fatalf("缺少锚点 %s", key)
		}
	}
}
