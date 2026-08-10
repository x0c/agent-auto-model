// Package anchors 检查当前已装 Agent 打包 JS 是否仍含挂钩锚点。
// 锚点字符串唯一来源：internal/assets/anchors.json。
package anchors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/agentbin"
	"github.com/x0c/cursor-mode-model/internal/assets"
)

// Result 锚点扫描结果。
type Result struct {
	AgentIndex string          `json:"agent_index"`
	Anchors    map[string]bool `json:"anchors"`
	OK         bool            `json:"ok"`
}

// Definitions 返回 name→源码片段。
func Definitions() map[string]string {
	out := map[string]string{}
	_ = json.Unmarshal(assets.AnchorsJSON(), &out)
	return out
}

// Check 扫描官方 Agent 版本目录。
func Check(home string) Result {
	defs := Definitions()
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
	ok := len(defs) > 0
	for name, needle := range defs {
		hit := needle != "" && strings.Contains(blob, needle)
		out.Anchors[name] = hit
		if !hit {
			ok = false
		}
	}
	out.OK = ok
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
