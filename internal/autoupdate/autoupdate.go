// Package autoupdate 提供基于 GitHub Releases 的静默自更新。
package autoupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/install"
	"github.com/x0c/agent-auto-model/internal/paths"
	"github.com/x0c/agent-auto-model/internal/recommended"
)

const (
	defaultRepo                  = "x0c/agent-auto-model"
	defaultChannel               = "github_release"
	defaultCheckIntervalHours    = 24
	envSkipCheck                 = "AGENT_AUTO_MODEL_SKIP_UPDATE_CHECK"
	envLatestReleaseURL          = "AGENT_AUTO_MODEL_UPDATE_LATEST_URL"
	envBackgroundUpdateChild     = "AGENT_AUTO_MODEL_BACKGROUND_UPDATE_CHILD"
	backgroundUpdatePollDuration = 5 * time.Second
)

var (
	nowFunc    = time.Now
	httpClient = &http.Client{Timeout: 8 * time.Second}
)

// State 记录最近一次自更新状态。
type State struct {
	InProgress           bool   `json:"in_progress"`
	LastCheckedAt        string `json:"last_checked_at,omitempty"`
	LastAttemptVersion   string `json:"last_attempt_version,omitempty"`
	LastInstalledVersion string `json:"last_installed_version,omitempty"`
	LastError            string `json:"last_error,omitempty"`
	ManagedBinary        string `json:"managed_binary,omitempty"`
}

// RuntimeStatus 用于 status 展示的运行态摘要。
type RuntimeStatus struct {
	Enabled            bool   `json:"enabled"`
	Channel            string `json:"channel"`
	CheckIntervalHours int    `json:"check_interval_hours"`
	InProgress         bool   `json:"in_progress"`
	LastCheckedAt      string `json:"last_checked_at,omitempty"`
	LastAttemptVersion string `json:"last_attempt_version,omitempty"`
	LastInstalled      string `json:"last_installed_version,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	ManagedBinary      string `json:"managed_binary,omitempty"`
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// MaybeCheckAndUpdate 按间隔执行一次静默自更新。
func MaybeCheckAndUpdate(home, currentVersion, executablePath string, force bool) error {
	recErr := recommended.MaybeRefresh(home, force)
	if recErr == nil {
		stored := config.Load(home)
		if stored.ModelsSource == config.ModelsSourceRecommended {
			_ = config.SyncRuntime(home, config.ApplyRecommended(home, stored))
		}
	}
	if os.Getenv(envSkipCheck) == "1" {
		return recErr
	}
	cfg := config.Load(home)
	if !cfg.AutoUpdate.Enabled {
		return recErr
	}
	target, managed, err := managedBinary(home, executablePath)
	if err != nil {
		return saveError(home, target, err)
	}
	if !managed {
		return saveState(home, mutateState(loadState(home), func(st *State) {
			st.ManagedBinary = target
			st.LastError = "skip unmanaged executable"
		}))
	}
	lock, err := acquireLock(home)
	if err != nil {
		return nil
	}
	defer releaseLock(lock)

	st := loadState(home)
	st.ManagedBinary = target
	if !force && withinCooldown(st, cfg.AutoUpdate.CheckIntervalHours) {
		return saveState(home, st)
	}
	st.InProgress = true
	st.LastCheckedAt = nowFunc().UTC().Format(time.RFC3339)
	st.LastError = ""
	if err := saveState(home, st); err != nil {
		return err
	}

	latest, err := fetchLatestRelease()
	if err != nil {
		return saveError(home, target, err)
	}
	st.LastAttemptVersion = latest.TagName
	if !isNewerVersion(currentVersion, latest.TagName) {
		st.InProgress = false
		st.LastError = ""
		return saveState(home, st)
	}

	assetURL, assetName, err := selectAsset(latest)
	if err != nil {
		return saveError(home, target, err)
	}
	tmpDir, err := os.MkdirTemp("", "agent-auto-model-update-*")
	if err != nil {
		return saveError(home, target, fmt.Errorf("创建临时目录失败: %w", err))
	}
	defer os.RemoveAll(tmpDir)

	downloadPath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(assetURL, downloadPath); err != nil {
		return saveError(home, target, err)
	}
	newBinary, err := extractBinary(downloadPath, tmpDir)
	if err != nil {
		return saveError(home, target, err)
	}
	canonical := canonicalBinaryPath(target)
	if err := replaceBinary(newBinary, canonical); err != nil {
		return saveError(home, target, err)
	}
	if canonical != target {
		_ = os.Remove(target)
		target = canonical
	}
	if _, err := install.Install(home, target, false, nil); err != nil {
		return saveError(home, target, fmt.Errorf("更新后二次 install 失败: %w", err))
	}
	st.InProgress = false
	st.LastInstalledVersion = latest.TagName
	st.LastError = ""
	return saveState(home, st)
}

// KickoffBackgroundCheck 启动后台子进程执行自更新，不阻塞主流程。
func KickoffBackgroundCheck(home, executablePath string) {
	if os.Getenv(envSkipCheck) == "1" || os.Getenv(envBackgroundUpdateChild) == "1" {
		return
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer devNull.Close()
	cmd := exec.Command(executablePath, "update", "--auto", "--quiet")
	cmd.Env = append(os.Environ(),
		envBackgroundUpdateChild+"=1",
	)
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = nil
	_ = cmd.Start()
}

// LoadRuntimeStatus 返回给 status 的自更新状态。
func LoadRuntimeStatus(home string) RuntimeStatus {
	cfg := config.Load(home)
	st := loadState(home)
	return RuntimeStatus{
		Enabled:            cfg.AutoUpdate.Enabled,
		Channel:            cfg.AutoUpdate.Channel,
		CheckIntervalHours: cfg.AutoUpdate.CheckIntervalHours,
		InProgress:         st.InProgress,
		LastCheckedAt:      st.LastCheckedAt,
		LastAttemptVersion: st.LastAttemptVersion,
		LastInstalled:      st.LastInstalledVersion,
		LastError:          st.LastError,
		ManagedBinary:      st.ManagedBinary,
	}
}

func fetchLatestRelease() (latestRelease, error) {
	url := os.Getenv(envLatestReleaseURL)
	if strings.TrimSpace(url) == "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", defaultRepo)
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return latestRelease{}, fmt.Errorf("查询最新 release 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return latestRelease{}, fmt.Errorf("查询最新 release 失败: http %d", resp.StatusCode)
	}
	var rel latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return latestRelease{}, fmt.Errorf("解析最新 release 响应失败: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return latestRelease{}, errors.New("最新 release 缺少 tag_name")
	}
	return rel, nil
}

func selectAsset(rel latestRelease) (string, string, error) {
	version := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	ext := ".tar.gz"
	allowAnyURL := strings.TrimSpace(os.Getenv(envLatestReleaseURL)) != ""
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	suffix := fmt.Sprintf("_%s_%s_%s%s", version, runtime.GOOS, goArchName(runtime.GOARCH), ext)
	want := "agent-auto-model" + suffix
	for _, asset := range rel.Assets {
		if asset.Name == want && (allowAnyURL || strings.HasPrefix(asset.URL, "https://github.com/")) {
			return asset.URL, asset.Name, nil
		}
	}
	return "", "", fmt.Errorf("未找到当前平台更新包 %s", want)
}

func goArchName(goarch string) string {
	switch goarch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}

func downloadFile(url, dst string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("下载更新包失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新包失败: http %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建更新包文件失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入更新包失败: %w", err)
	}
	return nil
}

func extractBinary(archivePath, tmpDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipBinary(archivePath, tmpDir)
	}
	return extractTarGzBinary(archivePath, tmpDir)
}

func extractTarGzBinary(archivePath, tmpDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("打开更新包失败: %w", err)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("读取 gzip 更新包失败: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", errors.New("更新包里未找到 agent-auto-model 二进制")
		}
		if err != nil {
			return "", fmt.Errorf("读取 tar 更新包失败: %w", err)
		}
		name := filepath.Base(hdr.Name)
		if !isOurBinaryName(name) {
			continue
		}
		out := filepath.Join(tmpDir, name)
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", fmt.Errorf("创建解压文件失败: %w", err)
		}
		if _, err := io.Copy(w, tr); err != nil {
			w.Close()
			return "", fmt.Errorf("解压二进制失败: %w", err)
		}
		if err := w.Close(); err != nil {
			return "", fmt.Errorf("关闭解压文件失败: %w", err)
		}
		return out, nil
	}
}

func extractZipBinary(archivePath, tmpDir string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("打开 zip 更新包失败: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !isOurBinaryName(name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("读取 zip 文件失败: %w", err)
		}
		out := filepath.Join(tmpDir, name)
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("创建解压文件失败: %w", err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			return "", fmt.Errorf("解压二进制失败: %w", err)
		}
		rc.Close()
		if err := w.Close(); err != nil {
			return "", fmt.Errorf("关闭解压文件失败: %w", err)
		}
		return out, nil
	}
	return "", errors.New("更新包里未找到 agent-auto-model 二进制")
}

func isOurBinaryName(name string) bool {
	switch name {
	case "agent-auto-model", "agent-auto-model.exe":
		return true
	default:
		return false
	}
}

func canonicalBinaryPath(target string) string {
	base := filepath.Base(target)
	leftover := paths.LeftoverCommandName()
	if base != leftover && base != leftover+".exe" {
		return target
	}
	name := paths.AppName
	if strings.HasSuffix(base, ".exe") {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(target), name)
}

func replaceBinary(src, target string) error {
	if runtime.GOOS == "windows" {
		return errors.New("windows 暂不支持运行中静默自更新")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	tmpTarget := target + ".tmp-update"
	if err := copyFile(src, tmpTarget, 0o755); err != nil {
		return fmt.Errorf("准备新二进制失败: %w", err)
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		_ = os.Remove(tmpTarget)
		return fmt.Errorf("替换二进制失败: %w", err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func managedBinary(home, executablePath string) (string, bool, error) {
	target := filepath.Clean(executablePath)
	if st, err := install.LoadState(home); err == nil && strings.TrimSpace(st.SelfBinary) != "" {
		target = filepath.Clean(st.SelfBinary)
	}
	absExec, err := filepath.Abs(executablePath)
	if err != nil {
		return target, false, fmt.Errorf("解析当前可执行文件路径失败: %w", err)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return target, false, fmt.Errorf("解析目标可执行文件路径失败: %w", err)
	}
	return absTarget, samePath(absExec, absTarget), nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func withinCooldown(st State, hours int) bool {
	if hours <= 0 || strings.TrimSpace(st.LastCheckedAt) == "" {
		return false
	}
	last, err := time.Parse(time.RFC3339, st.LastCheckedAt)
	if err != nil {
		return false
	}
	return nowFunc().Sub(last) < time.Duration(hours)*time.Hour
}

func acquireLock(home string) (*os.File, error) {
	path := paths.AutoUpdateLockFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("创建自更新锁失败: %w", err)
	}
	_, _ = f.WriteString(strconv.FormatInt(nowFunc().Unix(), 10))
	return f, nil
}

func releaseLock(f *os.File) {
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}

func loadState(home string) State {
	data, err := os.ReadFile(paths.AutoUpdateStateFile(home))
	if err != nil {
		return State{}
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}
	}
	return st
}

func saveState(home string, st State) error {
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	path := paths.AutoUpdateStateFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func saveError(home, target string, err error) error {
	st := loadState(home)
	st.InProgress = false
	st.ManagedBinary = target
	st.LastError = err.Error()
	if st.LastCheckedAt == "" {
		st.LastCheckedAt = nowFunc().UTC().Format(time.RFC3339)
	}
	if saveErr := saveState(home, st); saveErr != nil {
		return saveErr
	}
	return err
}

func mutateState(st State, fn func(*State)) State {
	fn(&st)
	return st
}

func isNewerVersion(current, latest string) bool {
	cur := parseVersion(current)
	newV := parseVersion(latest)
	for i := 0; i < 3; i++ {
		if newV[i] > cur[i] {
			return true
		}
		if newV[i] < cur[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			out[i] = n
		}
	}
	return out
}
