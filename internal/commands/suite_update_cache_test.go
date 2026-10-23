package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Obedience-Corp/obey-shared/updatecheck"
)

// useRealSuiteCheck swaps the recorders back to the shared package so a test
// exercises the real cache and display policy against a local release server.
func useRealSuiteCheck(t *testing.T, h *suiteHarness, srv *httptest.Server) {
	t.Helper()
	suiteNotify = updatecheck.MaybeNotify
	suiteCheck = updatecheck.Check
	suiteFetcher = &updatecheck.Fetcher{URL: srv.URL}
	t.Setenv("PATH", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(h.cachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
}

func seedSuiteCache(t *testing.T, path, latest string, checkedAt time.Time) {
	t.Helper()
	data, err := json.Marshal(updatecheck.Cache{
		SchemaVersion: updatecheck.SchemaVersion,
		Source:        updatecheck.Source,
		Channel:       updatecheck.ChannelStable,
		CheckedAt:     checkedAt.UTC(),
		LatestTag:     "v" + latest,
		LatestVersion: latest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readSuiteCache(t *testing.T, path string) updatecheck.Cache {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c updatecheck.Cache
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("cache %s: %v", path, err)
	}
	return c
}

func releaseServer(t *testing.T, tag string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAmbientNoticeFromFreshCacheOncePerRelease(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	var hits atomic.Int32
	useRealSuiteCheck(t, h, releaseServer(t, "v9.9.9", &hits))
	seedSuiteCache(t, h.cachePath(), "0.3.24", time.Now())
	probe := addProbe(t, h)

	_, first, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if !strings.Contains(first, "Festival suite update available (0.3.20 → 0.3.24)") {
		t.Fatalf("first run stderr missing notice:\n%s", first)
	}
	if !strings.Contains(first, "see:") || !strings.Contains(first, updatecheck.DefaultReleasePage) {
		t.Fatalf("first run stderr missing the action line:\n%s", first)
	}
	cache := readSuiteCache(t, h.cachePath())
	if cache.LastNotifiedVersion != "0.3.24" || cache.LastNotifiedAt.IsZero() {
		t.Fatalf("notified state not persisted: %+v", cache)
	}

	_, second, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second != "" {
		t.Fatalf("second run repeated the notice:\n%s", second)
	}
	if hits.Load() != 0 {
		t.Fatalf("fresh cache hit the release server %d times", hits.Load())
	}
}

func TestAmbientNoticeRefreshesStaleCache(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	var hits atomic.Int32
	useRealSuiteCheck(t, h, releaseServer(t, "v0.3.25", &hits))
	seedSuiteCache(t, h.cachePath(), "0.3.24", time.Now().Add(-48*time.Hour))
	probe := addProbe(t, h)

	_, stderr, err := runFest(t, probe)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("release server hits = %d, want 1", hits.Load())
	}
	if !strings.Contains(stderr, "(0.3.20 → 0.3.25)") {
		t.Fatalf("stderr does not name the refreshed release:\n%s", stderr)
	}
	if got := readSuiteCache(t, h.cachePath()).LatestVersion; got != "0.3.25" {
		t.Fatalf("cache latest_version = %q, want 0.3.25", got)
	}
}

// A release server that never answers must not hold the command past the
// ambient budget, must not fail it, and is not asked again on the next command.
func TestAmbientNoticeSlowNetworkStaysWithinBudget(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	var hits atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	useRealSuiteCheck(t, h, srv)
	probe := addProbe(t, h)

	for run := 1; run <= 2; run++ {
		start := time.Now()
		stdout, stderr, err := runFest(t, probe)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("run %d: slow network failed the command: %v", run, err)
		}
		if elapsed > updatecheck.AmbientTimeout+time.Second {
			t.Fatalf("run %d took %v with an unresponsive release server", run, elapsed)
		}
		if stdout != "probe output\n" || stderr != "" {
			t.Fatalf("run %d: stdout %q stderr %q", run, stdout, stderr)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("release server asked %d times in two commands, want 1", hits.Load())
	}
}
