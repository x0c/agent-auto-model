// Package agentbin 定位 Cursor Agent 官方可执行文件，避开本工具的包装层。
package agentbin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/x0c/cursor-mode-model/internal/paths"
)

// ErrNotFound 未安装 Cursor Agent。
var ErrNotFound = errors.New("cursor agent not found")

// Find 返回官方 cursor-agent 启动脚本的绝对路径。
func Find(home string) (string, error) {
	type cand struct {
		path    string
		modTime int64
		name    string
	}
	var list []cand
	names := []string{"cursor-agent"}
	if runtime.GOOS == "windows" {
		names = []string{"cursor-agent.cmd", "cursor-agent.exe", "cursor-agent.bat", "cursor-agent"}
	}
	for _, versions := range paths.CursorAgentVersionsDirs(home) {
		entries, err := os.ReadDir(versions)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			for _, name := range names {
				p := filepath.Join(versions, e.Name(), name)
				st, err := os.Stat(p)
				if err != nil || st.IsDir() {
					continue
				}
				list = append(list, cand{path: p, modTime: st.ModTime().UnixNano(), name: e.Name() + "/" + name})
			}
		}
	}
	if len(list) == 0 {
		return "", fmt.Errorf("%w\n%s", ErrNotFound, installHint())
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].modTime != list[j].modTime {
			return list[i].modTime > list[j].modTime
		}
		return list[i].name > list[j].name
	})
	return list[0].path, nil
}

func installHint() string {
	return "Cursor Agent CLI was not found. Install and log in first: https://cursor.com/install\n" +
		"未找到 Cursor Agent CLI。请先安装并登录：https://cursor.com/install"
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
