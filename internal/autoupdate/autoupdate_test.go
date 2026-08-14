package autoupdate

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/paths"
)

func TestMaybeCheckAndUpdateInstallsNewVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	execPath := filepath.Join(home, "agent-auto-model")
	if runtime.GOOS == "windows" {
		execPath += ".exe"
	}
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(home, config.Default()); err != nil {
		t.Fatal(err)
	}

	assetName := assetNameFor("1.2.3")
	archive := buildTarGz(t, binaryName(), []byte("new-binary"))
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v1.2.3",
				"assets": []map[string]string{{
					"name":                 assetName,
					"browser_download_url": serverURL + "/asset",
				}},
			})
		case "/asset":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	serverURL = srv.URL

	t.Setenv(envLatestReleaseURL, srv.URL+"/latest")
	prevNow, prevClient := nowFunc, httpClient
	nowFunc = func() time.Time { return time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC) }
	httpClient = srv.Client()
	defer func() {
		nowFunc = prevNow
		httpClient = prevClient
	}()

	if err := MaybeCheckAndUpdate(home, "0.2.0", execPath, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("更新后二进制不对: %q", string(data))
	}
	st := loadState(home)
	if st.LastInstalledVersion != "v1.2.3" {
		t.Fatalf("last installed = %q", st.LastInstalledVersion)
	}
	if st.LastError != "" {
		t.Fatalf("unexpected error: %q", st.LastError)
	}
}

func TestMaybeCheckAndUpdateHonorsCooldown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	execPath := filepath.Join(home, "agent-auto-model")
	if err := os.WriteFile(execPath, []byte("same"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.AutoUpdate.CheckIntervalHours = 24
	if err := config.Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	if err := saveState(home, State{
		LastCheckedAt: nowFunc().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv(envLatestReleaseURL, srv.URL+"/latest")
	prevNow, prevClient := nowFunc, httpClient
	nowFunc = func() time.Time { return time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC) }
	httpClient = srv.Client()
	defer func() {
		nowFunc = prevNow
		httpClient = prevClient
	}()

	if err := MaybeCheckAndUpdate(home, "0.2.0", execPath, false); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("冷却期内不应请求最新 release")
	}
}

func TestLoadRuntimeStatusReflectsConfigAndState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	cfg := config.Default()
	cfg.AutoUpdate.Enabled = false
	cfg.AutoUpdate.CheckIntervalHours = 12
	if err := config.Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	statePath := paths.AutoUpdateStateFile(home)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"in_progress":true,"last_checked_at":"2026-08-12T18:00:00Z","last_installed_version":"v0.2.1","last_error":"boom","managed_binary":"/tmp/cmm"}`
	if err := os.WriteFile(statePath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := LoadRuntimeStatus(home)
	if got.Enabled {
		t.Fatal("should be disabled")
	}
	if got.CheckIntervalHours != 12 {
		t.Fatalf("interval=%d", got.CheckIntervalHours)
	}
	if !got.InProgress || got.LastInstalled != "v0.2.1" || got.LastError != "boom" {
		t.Fatalf("unexpected status: %#v", got)
	}
}

func TestParseVersionAndComparison(t *testing.T) {
	if !isNewerVersion("0.2.0", "v0.2.1") {
		t.Fatal("expected newer version")
	}
	if isNewerVersion("0.3.0", "v0.2.9") {
		t.Fatal("should not downgrade")
	}
	if isNewerVersion("0.2.0", "v0.2.0-beta.1") {
		t.Fatal("pre-release should not beat same stable version")
	}
}

func assetNameFor(version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return "agent-auto-model_" + version + "_" + runtime.GOOS + "_" + goArchName(runtime.GOARCH) + ext
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "agent-auto-model.exe"
	}
	return "agent-auto-model"
}

func buildTarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf strings.Builder
	_ = buf
	tmp := filepath.Join(t.TempDir(), "archive.tar.gz")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(body)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
