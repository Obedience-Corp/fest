package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/version"
)

func TestWriteVersionInfo(t *testing.T) {
	tests := []struct {
		name        string
		info        version.Info
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name: "unstamped bundle omits the bundle line",
			info: version.Info{
				Version: "v0.6.2-5-gabcdef0",
				Commit:  "abcdef0",
				Profile: "stable",
			},
			wantAbsent:  []string{"bundle:"},
			wantPresent: []string{"fest v0.6.2-5-gabcdef0", "commit: abcdef0", "profile: stable"},
		},
		{
			name:        "whitespace-only fields still render their labels",
			info:        version.Info{Version: "dev", Profile: "dev"},
			wantAbsent:  []string{"bundle:"},
			wantPresent: []string{"fest dev", "commit: ", "built: ", "go: ", "platform: ", "profile: dev"},
		},
		{
			name: "festival bundle is reported on its own line",
			info: version.Info{
				Version: "v0.5.1",
				Bundle:  "v0.2.17",
				Commit:  "98b9950",
				Profile: "stable",
			},
			wantPresent: []string{"fest v0.5.1", "bundle: festival v0.2.17", "profile: stable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeVersionInfo(&buf, tt.info)
			got := buf.String()

			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("output contains %q, want it absent:\n%s", absent, got)
				}
			}
			for _, present := range tt.wantPresent {
				if !strings.Contains(got, present) {
					t.Errorf("output missing %q:\n%s", present, got)
				}
			}
		})
	}
}

func TestWriteVersionInfoLineOrder(t *testing.T) {
	var buf bytes.Buffer
	writeVersionInfo(&buf, version.Info{
		Version:   "v0.5.1",
		Bundle:    "v0.2.17",
		Commit:    "98b9950",
		BuildDate: "2026-08-21T00:00:00Z",
		GoVersion: "go1.24.0",
		Platform:  "darwin/arm64",
		Profile:   "stable",
	})

	want := []string{
		"fest v0.5.1",
		"bundle: festival v0.2.17",
		"commit: 98b9950",
		"built: 2026-08-21T00:00:00Z",
		"go: go1.24.0",
		"platform: darwin/arm64",
		"profile: stable",
	}
	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")

	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), buf.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
