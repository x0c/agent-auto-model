// Package recommended 拉取、缓存并校验仓库里的推荐模型映射。
package recommended

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/runtime/codex/spec"
)

const (
	defaultURL           = "https://raw.githubusercontent.com/x0c/agent-auto-model/main/recommended-models.json"
	defaultCheckInterval = 6 * time.Hour
	defaultTimeout       = 8 * time.Second
	installTimeout       = 3 * time.Second
	maxBodyBytes         = 64 * 1024

	SourceRemote = "remote"
	SourceCache  = "cache"
	SourceEmbed  = "embed"

	runtimeCursor = "cursor"
	runtimeCodex  = "codex"
)

// File 仓库推荐映射的格式。
type File struct {
	SchemaVersion int                      `json:"schema_version"`
	Runtimes      map[string]RuntimeModels `json:"runtimes"`
}

// RuntimeModels 单个 runtime 的推荐映射。
type RuntimeModels struct {
	Models map[string]string `json:"models"`
}

// Cache 本机缓存（含 ETag 与检查时间）。
type Cache struct {
	ETag          string `json:"etag,omitempty"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Source        string `json:"source,omitempty"`
	File
}

// RuntimeStatus 给 status / config show 的摘要。
type RuntimeStatus struct {
	Source        string `json:"source"`
	SourceTag     string `json:"source_tag"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	URL           string `json:"url"`
	ETag          string `json:"etag,omitempty"`
}

var requiredModes = map[string][]string{
	runtimeCursor: {"plan", "default", "search", "debug"},
	runtimeCodex:  {"plan", "default"},
}

var (
	nowFunc    = time.Now
	httpClient = &http.Client{
		Timeout:       defaultTimeout,
		CheckRedirect: httpsOnlyRedirect,
	}
)

//go:embed recommended-models.json
var embeddedJSON []byte

var embeddedFile File

func init() {
	f, err := Parse(embeddedJSON)
	if err != nil {
		panic("内置推荐配置无效: " + err.Error())
	}
	embeddedFile = f
}

// Embedded 返回安装包内置的推荐映射。
func Embedded() File {
	return cloneFile(embeddedFile)
}

// EmbeddedJSON 返回内置文件原文，供与仓库根文件对照。
func EmbeddedJSON() []byte {
	return append([]byte(nil), embeddedJSON...)
}

// Models 返回指定 runtime 的映射副本；未知 runtime 返回空 map。
func (f File) Models(runtime string) map[string]string {
	if f.Runtimes == nil {
		return map[string]string{}
	}
	rt, ok := f.Runtimes[runtime]
	if !ok || rt.Models == nil {
		return map[string]string{}
	}
	return copyMap(rt.Models)
}

// Parse 解析并校验推荐文件。
func Parse(data []byte) (File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("解析推荐配置失败: %w", err)
	}
	if err := Validate(f); err != nil {
		return File{}, err
	}
	return f, nil
}

// Validate 检查格式版本、必填 Mode、模型 ID。
func Validate(f File) error {
	if f.SchemaVersion < 1 {
		return errors.New("推荐配置缺少有效的 schema_version")
	}
	if f.Runtimes == nil {
		return errors.New("推荐配置缺少 runtimes")
	}
	for runtime, modes := range requiredModes {
		rt, ok := f.Runtimes[runtime]
		if !ok || rt.Models == nil {
			return fmt.Errorf("推荐配置缺少 %s 映射", runtime)
		}
		for _, mode := range modes {
			id := strings.TrimSpace(rt.Models[mode])
			if id == "" {
				return fmt.Errorf("推荐配置 %s.%s 为空", runtime, mode)
			}
			if runtime == runtimeCodex {
				s := spec.Parse(id)
				if s.Empty() {
					return fmt.Errorf("推荐配置 %s.%s 不是合法的 Codex 规格", runtime, mode)
				}
				if !spec.ValidEffort(s.Effort) {
					return fmt.Errorf("推荐配置 %s.%s 的 effort 非法", runtime, mode)
				}
			}
		}
	}
	return nil
}

// Equal 比较两份推荐映射的模型是否一致。
func Equal(a, b File) bool {
	for runtime, modes := range requiredModes {
		am := a.Models(runtime)
		bm := b.Models(runtime)
		for _, mode := range modes {
			if am[mode] != bm[mode] {
				return false
			}
		}
	}
	return true
}

// Resolve 返回本机可用的推荐映射：有效缓存优先，否则内置。
func Resolve(home string) File {
	c := loadCache(home)
	if err := Validate(c.File); err == nil {
		return cloneFile(c.File)
	}
	return Embedded()
}

// Status 返回缓存与来源摘要。
func Status(home string) RuntimeStatus {
	c := loadCache(home)
	source := c.Source
	if source == "" {
		if err := Validate(c.File); err == nil {
			source = SourceCache
		} else {
			source = SourceEmbed
		}
	}
	if err := Validate(c.File); err != nil {
		source = SourceEmbed
	}
	return RuntimeStatus{
		Source:        source,
		SourceTag:     sourceTag(source),
		LastCheckedAt: c.LastCheckedAt,
		LastError:     c.LastError,
		URL:           fetchURL(),
		ETag:          c.ETag,
	}
}

// MaybeRefresh 按间隔用 ETag 检查远程推荐配置。force 时忽略间隔仍带 ETag。
func MaybeRefresh(home string, force bool) error {
	return refresh(home, force, defaultTimeout)
}

// RefreshAtInstall 安装时试拉一次，失败不返回错误。
func RefreshAtInstall(home string) {
	_ = refresh(home, true, installTimeout)
}

func refresh(home string, force bool, timeout time.Duration) error {
	if skipNetwork() {
		return nil
	}
	lock, err := acquireLock(home)
	if err != nil {
		return nil
	}
	defer releaseLock(lock)

	c := loadCache(home)
	if !force && withinCooldown(c.LastCheckedAt) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL(), nil)
	if err != nil {
		return saveError(home, c, fmt.Errorf("构造推荐配置请求失败: %w", err))
	}
	req.Header.Set("User-Agent", "agent-auto-model")
	if strings.TrimSpace(c.ETag) != "" {
		req.Header.Set("If-None-Match", c.ETag)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return saveError(home, c, fmt.Errorf("拉取推荐配置失败: %w", err))
	}
	defer resp.Body.Close()

	c.LastCheckedAt = nowFunc().UTC().Format(time.RFC3339)
	switch resp.StatusCode {
	case http.StatusNotModified:
		c.LastError = ""
		if c.Source == "" {
			c.Source = SourceCache
		}
		return saveCache(home, c)
	case http.StatusOK:
		limited := io.LimitReader(resp.Body, maxBodyBytes+1)
		body, err := io.ReadAll(limited)
		if err != nil {
			return saveError(home, c, fmt.Errorf("读取推荐配置失败: %w", err))
		}
		if len(body) > maxBodyBytes {
			return saveError(home, c, errors.New("推荐配置超过大小上限"))
		}
		f, err := Parse(body)
		if err != nil {
			return saveError(home, c, err)
		}
		c.File = f
		c.ETag = strings.TrimSpace(resp.Header.Get("ETag"))
		c.LastError = ""
		c.Source = SourceRemote
		return saveCache(home, c)
	default:
		return saveError(home, c, fmt.Errorf("拉取推荐配置失败: http %d", resp.StatusCode))
	}
}

func fetchURL() string {
	if u := strings.TrimSpace(os.Getenv(paths.EnvRecommendedURL)); u != "" {
		return u
	}
	return defaultURL
}

func skipNetwork() bool {
	if os.Getenv(paths.EnvSkipRecommendedCheck) == "1" {
		return true
	}
	if strings.TrimSpace(os.Getenv(paths.EnvRecommendedURL)) != "" {
		return false
	}
	if os.Getenv("AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK") == "1" {
		return true
	}
	name := filepath.Base(os.Args[0])
	return strings.Contains(name, ".test")
}

func withinCooldown(lastChecked string) bool {
	if strings.TrimSpace(lastChecked) == "" {
		return false
	}
	last, err := time.Parse(time.RFC3339, lastChecked)
	if err != nil {
		return false
	}
	return nowFunc().Sub(last) < defaultCheckInterval
}

func loadCache(home string) Cache {
	data, err := os.ReadFile(paths.RecommendedCacheFile(home))
	if err != nil {
		return Cache{}
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return Cache{}
	}
	return c
}

func saveCache(home string, c Cache) error {
	payload, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := paths.RecommendedCacheFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func saveError(home string, c Cache, err error) error {
	c.LastCheckedAt = nowFunc().UTC().Format(time.RFC3339)
	c.LastError = err.Error()
	_ = saveCache(home, c)
	return err
}

func acquireLock(home string) (*os.File, error) {
	path := paths.RecommendedLockFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func releaseLock(f *os.File) {
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}

func httpsOnlyRedirect(req *http.Request, _ []*http.Request) error {
	if req.URL.Scheme != "https" && req.URL.Hostname() != "127.0.0.1" && req.URL.Hostname() != "localhost" {
		return errors.New("拒绝跳转到非 HTTPS 地址")
	}
	return nil
}

func sourceTag(source string) string {
	switch source {
	case SourceRemote:
		return "远程"
	case SourceCache:
		return "本机缓存"
	default:
		return "安装包内置"
	}
}

func cloneFile(in File) File {
	out := File{SchemaVersion: in.SchemaVersion, Runtimes: map[string]RuntimeModels{}}
	for name, rt := range in.Runtimes {
		out.Runtimes[name] = RuntimeModels{Models: copyMap(rt.Models)}
	}
	return out
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
