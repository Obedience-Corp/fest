package festival

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestMarkerResultFromContent_ListsReplaceMarkers(t *testing.T) {
	got := markerResultFromContent("# Goal\n\n[REPLACE: Describe the sequence objective]\n- [ ] [REPLACE: Task 1]\n")
	if got.Total != 2 {
		t.Fatalf("Total = %d, want 2", got.Total)
	}
	if len(got.Markers) != 2 {
		t.Fatalf("len(Markers) = %d, want 2", len(got.Markers))
	}
	hint, _ := got.Markers[0]["hint"].(string)
	if hint != "Describe the sequence objective" {
		t.Fatalf("first hint = %q", hint)
	}
}

func TestMarkerResultFromContent_Empty(t *testing.T) {
	got := markerResultFromContent("# Goal\nno markers here\n")
	if got.Total != 0 || got.Markers != nil {
		t.Fatalf("expected empty result, got %+v", got)
	}
}

func TestEmitCreateDryRun_JSONIncludesPlannedPaths(t *testing.T) {
	stdout, restore := captureStdout(t)

	err := emitCreateDryRun(true, []string{"01_hello/SEQUENCE_GOAL.md"}, markerResultFromContent("[REPLACE: x]"))
	restore()
	if err != nil {
		t.Fatalf("emitCreateDryRun: %v", err)
	}

	var payload struct {
		OK           bool             `json:"ok"`
		Action       string           `json:"action"`
		DryRun       bool             `json:"dry_run"`
		PlannedPaths []string         `json:"planned_paths"`
		Count        int              `json:"count"`
		Markers      []map[string]any `json:"markers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if !payload.OK || !payload.DryRun || payload.Action != "dry_run" {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.PlannedPaths) != 1 || payload.PlannedPaths[0] != "01_hello/SEQUENCE_GOAL.md" {
		t.Fatalf("planned_paths = %v", payload.PlannedPaths)
	}
	if payload.Count != 1 {
		t.Fatalf("count = %d", payload.Count)
	}
}

func captureStdout(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	buf := &bytes.Buffer{}
	return buf, func() {
		_ = w.Close()
		os.Stdout = old
		_, _ = io.Copy(buf, r)
		_ = r.Close()
	}
}
