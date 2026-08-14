package recommended

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/x0c/agent-auto-model/internal/paths"
)

func TestEmbeddedValid(t *testing.T) {
	t.Parallel()
	if err := Validate(Embedded()); err != nil {
		t.Fatal(err)
	}
	if Embedded().Models("cursor")["plan"] == "" {
		t.Fatal("内置 cursor.plan 为空")
	}
}

func TestPublicFileMatchesEmbed(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "recommended-models.json")
	data, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !Equal(pub, Embedded()) {
		t.Fatal("仓库根 recommended-models.json 与内置文件不一致")
	}
}

func TestParseRejectsEmptyMode(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte(`{"schema_version":1,"runtimes":{"cursor":{"models":{"plan":""}},"codex":{"models":{"plan":"x","default":"y"}}}}`))
	if err == nil {
		t.Fatal("空 plan 应失败")
	}
}

func TestMaybeRefreshETagAndUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	payload := Embedded()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(paths.EnvRecommendedURL, srv.URL)

	if err := MaybeRefresh(home, true); err != nil {
		t.Fatal(err)
	}
	got := Resolve(home)
	if !Equal(got, payload) {
		t.Fatalf("缓存未写入: %#v", got)
	}
	st := Status(home)
	if st.Source != SourceRemote || st.ETag != `"v1"` {
		t.Fatalf("status=%#v", st)
	}

	if err := MaybeRefresh(home, true); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("hits=%d", hits)
	}
	if Status(home).Source != SourceRemote {
		t.Fatalf("304 后 source=%s", Status(home).Source)
	}
}

func TestMaybeRefreshHonorsCooldown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("ETag", `"x"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(EmbeddedJSON())
	}))
	t.Cleanup(srv.Close)
	t.Setenv(paths.EnvRecommendedURL, srv.URL)

	if err := MaybeRefresh(home, true); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRefresh(home, false); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("冷却期内不应再请求: hits=%d", hits)
	}
}

func TestMaybeRefreshKeepsCacheOnInvalidBody(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"good"`)
		_, _ = w.Write(EmbeddedJSON())
	}))
	t.Setenv(paths.EnvRecommendedURL, good.URL)
	if err := MaybeRefresh(home, true); err != nil {
		t.Fatal(err)
	}
	good.Close()

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"schema_version":1,"runtimes":{}}`))
	}))
	t.Cleanup(bad.Close)
	t.Setenv(paths.EnvRecommendedURL, bad.URL)
	if err := MaybeRefresh(home, true); err == nil {
		t.Fatal("非法正文应失败")
	}
	if !Equal(Resolve(home), Embedded()) {
		t.Fatal("校验失败应保留旧缓存")
	}
	if !strings.Contains(Status(home).LastError, "缺少") && Status(home).LastError == "" {
		t.Fatalf("应记录错误: %#v", Status(home))
	}
}

func TestResolveFallsBackToEmbed(t *testing.T) {
	t.Parallel()
	got := Resolve(t.TempDir())
	if !Equal(got, Embedded()) {
		t.Fatal("无缓存时应回落内置")
	}
}

func TestCooldownClock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(EmbeddedJSON())
	}))
	t.Cleanup(srv.Close)
	t.Setenv(paths.EnvRecommendedURL, srv.URL)

	base := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base }
	t.Cleanup(func() { nowFunc = time.Now })
	if err := MaybeRefresh(home, true); err != nil {
		t.Fatal(err)
	}
	nowFunc = func() time.Time { return base.Add(defaultCheckInterval + time.Second) }
	if err := MaybeRefresh(home, false); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("过期后应再请求: hits=%d", hits)
	}
}
