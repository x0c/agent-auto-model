// Package runtime 定义多 Agent runtime 的共同接口。
package runtime

import "github.com/x0c/agent-auto-model/internal/config"

// Info 描述一个可安装的 Agent runtime。
type Info struct {
	Name         string
	WrapperNames []string
	ValidModes   []string
}

// Cursor Cursor Agent CLI。
var Cursor = Info{
	Name:         config.RuntimeCursor,
	WrapperNames: []string{"agent", "cursor-agent"},
	ValidModes:   config.ValidCursorModes,
}

// Codex OpenAI Codex CLI。
var Codex = Info{
	Name:         config.RuntimeCodex,
	WrapperNames: []string{"codex"},
	ValidModes:   config.ValidCodexModes,
}

// All 已接入 runtime，安装顺序即 PATH 包装写入顺序。
var All = []Info{Cursor, Codex}

// ByName 按名字查找。
func ByName(name string) (Info, bool) {
	for _, r := range All {
		if r.Name == name {
			return r, true
		}
	}
	return Info{}, false
}

// Filter 按名字列表过滤；空列表表示全部。
func Filter(names []string) []Info {
	if len(names) == 0 {
		return append([]Info{}, All...)
	}
	out := make([]Info, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		if info, ok := ByName(name); ok {
			out = append(out, info)
			seen[name] = true
		}
	}
	return out
}

// IsWrapperName 判断 invoked-as 是否属于某个 runtime。
func IsWrapperName(name string) bool {
	for _, r := range All {
		for _, w := range r.WrapperNames {
			if w == name {
				return true
			}
		}
	}
	return false
}

// RuntimeForWrapper 根据包装入口名返回 runtime。
func RuntimeForWrapper(name string) (Info, bool) {
	for _, r := range All {
		for _, w := range r.WrapperNames {
			if w == name {
				return r, true
			}
		}
	}
	return Info{}, false
}
