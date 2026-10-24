package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Obedience-Corp/obey-shared/updatecheck"
)

func TestVersionCheckEmptyBundleDoesNoIO(t *testing.T) {
	h := stubSuiteNotice(t, "")

	stdout, _, err := runFest(t, "version", "--check")
	if err != nil {
		t.Fatalf("version --check: %v", err)
	}
	if !strings.Contains(stdout, "festival suite update check skipped: not a festival suite release build") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if h.checks != 0 {
		t.Fatalf("empty bundle called the checker %d times", h.checks)
	}
	if _, err := os.Stat(h.obeyDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty bundle touched %s (stat err %v)", h.obeyDir, err)
	}

	stdout, _, err = runFest(t, "version", "--check", "--json")
	if err != nil {
		t.Fatalf("version --check --json: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if rep["skip_reason"] != suiteSkipNotRelease || rep["update_available"] != false || rep["current_version"] != "" {
		t.Fatalf("empty-bundle report: %v", rep)
	}
	if _, ok := rep["latest_version"]; ok {
		t.Fatalf("empty-bundle report names a latest version: %v", rep)
	}
}

func TestVersionCheckNoHomeSkips(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	suiteObeyDir = func() (string, error) { return "", errors.New("no home") }

	stdout, _, err := runFest(t, "version", "--check")
	if err != nil {
		t.Fatalf("version --check: %v", err)
	}
	if !strings.Contains(stdout, "(festival suite 0.3.20)") || !strings.Contains(stdout, "skipped: cannot determine an absolute home directory") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if h.checks != 0 {
		t.Fatalf("no home still called the checker %d times", h.checks)
	}
}

func TestVersionCheckReportsUpdateFromCache(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.20")
	var hits atomic.Int32
	useRealSuiteCheck(t, h, releaseServer(t, "v9.9.9", &hits))
	seedSuiteCache(t, h.cachePath(), "0.3.24", time.Now())

	stdout, stderr, err := runFest(t, "version", "--check")
	if err != nil {
		t.Fatalf("version --check: %v", err)
	}
	for _, want := range []string{
		" (festival suite 0.3.20)\n",
		"festival suite latest: 0.3.24\n",
		"update available\n",
		"see: " + updatecheck.DefaultReleasePage + "\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if stderr != "" {
		t.Fatalf("version --check also printed an ambient notice:\n%s", stderr)
	}
	if hits.Load() != 0 {
		t.Fatalf("fresh cache hit the release server %d times", hits.Load())
	}

	stdout, _, err = runFest(t, "version", "--check", "--json")
	if err != nil {
		t.Fatalf("version --check --json: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	want := map[string]any{
		"schema_version":   "version-check/v1alpha1",
		"product":          "fest",
		"current_version":  "0.3.20",
		"latest_version":   "0.3.24",
		"latest_tag":       "v0.3.24",
		"source":           updatecheck.Source,
		"update_available": true,
		"upgrade_kind":     "see",
		"upgrade_command":  updatecheck.DefaultReleasePage,
	}
	for k, v := range want {
		if rep[k] != v {
			t.Errorf("%s = %v, want %v", k, rep[k], v)
		}
	}
	if cv, _ := rep["component_version"].(string); cv == "" || cv == "0.3.20" || cv == "v0.3.20" {
		t.Errorf("component_version = %v, want fest's own version, not the suite's", rep["component_version"])
	}
	if _, err := time.Parse(time.RFC3339, rep["checked_at"].(string)); err != nil {
		t.Errorf("checked_at %v is not RFC3339: %v", rep["checked_at"], err)
	}
	if got := readSuiteCache(t, h.cachePath()); !got.LastNotifiedAt.IsZero() {
		t.Fatalf("--check wrote ambient notified state: %+v", got)
	}
}

// An old cache and a failed refresh must say the answer is stale rather than
// claim the build is current.
func TestVersionCheckStaleCacheReportsFailedRefresh(t *testing.T) {
	h := stubSuiteNotice(t, "v0.3.24")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	useRealSuiteCheck(t, h, srv)
	checked := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
	seedSuiteCache(t, h.cachePath(), "0.3.24", checked)

	stdout, _, err := runFest(t, "version", "--check")
	if err != nil {
		t.Fatalf("version --check: %v", err)
	}
	want := "festival suite latest: 0.3.24 (cached " + checked.Format(time.RFC3339) + "; refresh from GitHub failed)\n"
	if !strings.Contains(stdout, want) || !strings.Contains(stdout, "up to date\n") {
		t.Fatalf("stdout missing %q:\n%s", want, stdout)
	}

	stdout, _, err = runFest(t, "version", "--check", "--json")
	if err != nil {
		t.Fatalf("version --check --json: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if rep["from_cache"] != true || rep["refresh_error"] == nil || rep["refresh_error"] == "" {
		t.Fatalf("stale report hides the failed refresh: %v", rep)
	}
	if hits.Load() != 2 {
		t.Fatalf("release server hits = %d, want one per explicit check", hits.Load())
	}
}

func TestVersionCheckTakesPrecedenceOverShort(t *testing.T) {
	stubSuiteNotice(t, "")
	stdout, _, err := runFest(t, "version", "--check", "--short")
	if err != nil {
		t.Fatalf("version --check --short: %v", err)
	}
	if !strings.Contains(stdout, "festival suite update check skipped") {
		t.Fatalf("--short won over --check; stdout:\n%s", stdout)
	}
}

func TestWriteVersionCheck(t *testing.T) {
	tests := []struct {
		name   string
		info   versionCheckInfo
		want   []string
		absent []string
	}{
		{
			name:   "not a suite build",
			info:   versionCheckInfo{ComponentVersion: "v0.9.3", SkipReason: suiteSkipNotRelease},
			want:   []string{"fest v0.9.3\n", "festival suite update check skipped: not a festival suite release build\n"},
			absent: []string{"latest"},
		},
		{
			name:   "check failed",
			info:   versionCheckInfo{ComponentVersion: "v0.9.3", CurrentVersion: "0.3.20", Source: updatecheck.Source},
			want:   []string{"fest v0.9.3 (festival suite 0.3.20)\n", "festival suite latest: unknown (could not refresh from GitHub)\n"},
			absent: []string{"up to date", "update available"},
		},
		{
			name:   "up to date",
			info:   versionCheckInfo{ComponentVersion: "v0.9.3", CurrentVersion: "0.3.24", LatestVersion: "0.3.24"},
			want:   []string{"festival suite latest: 0.3.24\n", "up to date\n"},
			absent: []string{"run:", "see:"},
		},
		{
			name: "ahead of the latest release",
			info: versionCheckInfo{ComponentVersion: "v0.9.3", CurrentVersion: "0.3.25", LatestVersion: "0.3.24"},
			want: []string{"newer than the latest festival suite release\n"},
		},
		{
			name: "update available",
			info: versionCheckInfo{ComponentVersion: "v0.9.3", CurrentVersion: "0.3.20", LatestVersion: "0.3.24", UpdateAvailable: true, UpgradeKind: "run", UpgradeCommand: "festival update"},
			want: []string{"update available\n", "run: festival update\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeVersionCheck(&buf, tt.info)
			got := buf.String()
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q:\n%s", w, got)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(got, a) {
					t.Errorf("unexpected %q:\n%s", a, got)
				}
			}
		})
	}
}
