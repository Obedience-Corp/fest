package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestWarnProgressDeprecation(t *testing.T) {
	tests := []struct {
		name     string
		opts     *progressOptions
		wantWarn bool
		wantHas  string
	}{
		{name: "complete_redirects_to_task_completed", opts: &progressOptions{complete: true}, wantWarn: true, wantHas: "fest task completed --yes"},
		{name: "update_redirects_to_task_update", opts: &progressOptions{update: "50%"}, wantWarn: true, wantHas: "fest task update"},
		{name: "blocker_redirects_to_task_blocked", opts: &progressOptions{blocker: "stuck"}, wantWarn: true, wantHas: "fest task blocked"},
		{name: "clear_redirects_to_task_unblock", opts: &progressOptions{clear: true}, wantWarn: true, wantHas: "fest task unblock"},
		{name: "in_progress_not_deprecated", opts: &progressOptions{inProgress: true}, wantWarn: false},
		{name: "read_only_not_deprecated", opts: &progressOptions{}, wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnProgressDeprecation(&buf, tt.opts)
			got := buf.String()

			if tt.wantWarn {
				if !strings.Contains(got, "deprecated") {
					t.Errorf("expected a deprecation warning, got %q", got)
				}
				if !strings.Contains(got, tt.wantHas) {
					t.Errorf("warning = %q, want it to name %q", got, tt.wantHas)
				}
			} else if got != "" {
				t.Errorf("expected no warning, got %q", got)
			}
		})
	}
}
