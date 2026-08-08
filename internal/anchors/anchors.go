// Package anchors 检查当前已装 Agent 打包 JS 是否仍含挂钩锚点。
package anchors

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/agentbin"
)

const (
	SetCurrentModel       = "setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){"
	SetCurrentModelParams = "setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){"
	SetModelFromStoredID  = "setModelFromStoredId(e,t){return p(this,void 0,void 0,(function*(){"
	GetCurrentModel       = "getCurrentModel(){return this.deriveCurrentModelDetails()"
	SetMetadata           = "setMetadata(e,t){this.metadataStore.set(e,t)}"
)

// Result 锚点扫描结果。
type Result struct {
	AgentIndex string          `json:"agent_index"`
	Anchors    map[string]bool `json:"anchors"`
	OK         bool            `json:"ok"`
}

// Check 扫描官方 Agent 版本目录。
func Check(home string) Result {
	out := Result{Anchors: map[string]bool{}}
	agent, err := agentbin.Find(home)
	if err != nil {
		return out
	}
	index := agentbin.IndexJS(agent)
	out.AgentIndex = index
	blob, err := readBundle(filepath.Dir(agent), index)
	if err != nil {
		return out
	}
	out.Anchors["setCurrentModel"] = strings.Contains(blob, SetCurrentModel)
	out.Anchors["setCurrentModelWithParameters"] = strings.Contains(blob, SetCurrentModelParams)
	out.Anchors["setModelFromStoredId"] = strings.Contains(blob, SetModelFromStoredID)
	out.Anchors["getCurrentModel"] = strings.Contains(blob, GetCurrentModel)
	out.Anchors["setMetadata"] = strings.Contains(blob, SetMetadata)
	out.OK = out.Anchors["setCurrentModel"] &&
		out.Anchors["setCurrentModelWithParameters"] &&
		out.Anchors["setModelFromStoredId"] &&
		out.Anchors["getCurrentModel"] &&
		out.Anchors["setMetadata"]
	return out
}

func readBundle(dir, index string) (string, error) {
	var b strings.Builder
	if index != "" {
		data, err := os.ReadFile(index)
		if err == nil {
			b.Write(data)
			b.WriteByte('\n')
		}
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.index.js"))
	if err != nil {
		return b.String(), nil
	}
	type file struct {
		path string
		size int64
	}
	var files []file
	for _, p := range entries {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		files = append(files, file{path: p, size: st.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].size > files[j].size })
	if len(files) > 8 {
		files = files[:8]
	}
	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
