package show

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// fest show puts the deferred blocker count recorded on a gate approval next
// to the judge verdict. An approval recorded before the count existed renders
// exactly as it did.
func TestShowRendersDeferredBlockerCountOnJudgeApproval(t *testing.T) {
	festDir := setupShowGateFestival(t)

	// Raw ledger lines, as a festival on disk carries them. Step 1 was
	// approved with the count; step 2 is an older approval without it.
	lines := []string{
		`{"ts":"2026-10-20T07:00:00Z","event":"completed","task":"001_IMPLEMENT/01_sequence/01_task.md"}`,
		`{"ts":"2026-10-20T07:01:00Z","event":"wf_init","phase":"gate:001_IMPLEMENT","total_steps":2}`,
		`{"ts":"2026-10-20T07:01:01Z","event":"wf_step_start","phase":"gate:001_IMPLEMENT","step":1}`,
		`{"ts":"2026-10-20T07:01:02Z","event":"wf_judge_started","phase":"gate:001_IMPLEMENT","step":1,"judge_status":"running","judge_command":"judge","judge_run_id":"r1"}`,
		`{"ts":"2026-10-20T07:01:03Z","event":"wf_judge_returned","phase":"gate:001_IMPLEMENT","step":1,"judge_status":"approved","judge_detail":"ok","judge_run_id":"r1"}`,
		`{"ts":"2026-10-20T07:01:04Z","event":"wf_step_done","phase":"gate:001_IMPLEMENT","step":1,"feedback":"ok","decision_actor":"agent","decision_summary":"ok","deferred_blockers":4}`,
		`{"ts":"2026-10-20T07:01:05Z","event":"wf_advance","phase":"gate:001_IMPLEMENT","step":2}`,
		`{"ts":"2026-10-20T07:01:06Z","event":"wf_step_start","phase":"gate:001_IMPLEMENT","step":2}`,
		`{"ts":"2026-10-20T07:01:07Z","event":"wf_judge_started","phase":"gate:001_IMPLEMENT","step":2,"judge_status":"running","judge_command":"judge","judge_run_id":"r2"}`,
		`{"ts":"2026-10-20T07:01:08Z","event":"wf_judge_returned","phase":"gate:001_IMPLEMENT","step":2,"judge_status":"approved","judge_detail":"ok","judge_run_id":"r2"}`,
		`{"ts":"2026-10-20T07:01:09Z","event":"wf_step_done","phase":"gate:001_IMPLEMENT","step":2,"feedback":"ok","decision_actor":"agent","decision_summary":"ok"}`,
	}
	festState := filepath.Join(festDir, progress.ProgressDir)
	if err := os.MkdirAll(festState, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(festState, progress.ProgressEventsFile), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	tree, err := BuildFestivalTree(context.Background(), festDir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}
	out := RenderTree(tree, DefaultTreeOptions())

	if !strings.Contains(out, "Judge: approved (4 deferred blockers)") {
		t.Fatalf("show must put the deferred blocker count on the judge line:\n%s", out)
	}
	if strings.Count(out, "Judge: approved") != 2 || strings.Count(out, "deferred blocker") != 1 {
		t.Fatalf("the older approval must render as a plain judge line:\n%s", out)
	}

	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"deferred_blockers":4`) {
		t.Fatalf("fest show --json must carry the count on the gate step: %s", raw)
	}
}
