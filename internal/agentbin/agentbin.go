// Package agentbin 定位 Cursor Agent 官方可执行文件，避开本工具的包装层。
package agentbin

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"forgejo.caozc.top/Max/cursor-mode-model/internal/paths"
)

// Find 返回官方 cursor-agent 启动脚本的绝对路径。
func Find(home string) (string, error) {
	versions := paths.CursorAgentVersionsDir(home)
	entries, err := os.ReadDir(versions)
	if err != nil {
		return "", err
	}
	type cand struct {
		path    string
		modTime int64
		name    string
	}
	var list []cand
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(versions, e.Name(), "cursor-agent")
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		list = append(list, cand{path: p, modTime: st.ModTime().UnixNano(), name: e.Name()})
	}
	if len(list) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].modTime != list[j].modTime {
			return list[i].modTime > list[j].modTime
		}
		return list[i].name > list[j].name
	})
	return list[0].path, nil
}

// IndexJS 返回同版本目录下的 index.js。
func IndexJS(agentPath string) string {
	if agentPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(agentPath), "index.js")
}

// IsOurWrapper 判断 path 是否落在本工具的 wrapper bin 目录。
func IsOurWrapper(home, path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	prefix := paths.WrapperBinDir(home) + string(os.PathSeparator)
	return strings.HasPrefix(abs, prefix) || abs == paths.WrapperBinDir(home)
}
