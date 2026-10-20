package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
)

const gateBlockerPhase = "001_IMPLEMENT"

// seededTask is one task file plus the progress state written for it.
// state is completed, in_progress, blocked, deferred, or pending.
type seededTask struct {
	name        string
	state       string
	reason      string
	deferReason string
}

// seedPhaseTasks writes the task files of one sequence and records their state
// through the progress manager, the same path fest task commands use.
func seedPhaseTasks(t *testing.T, festDir, phase, seq string, tasks []seededTask) {
	t.Helper()
	ctx := context.Background()
	seqDir := filepath.Join(festDir, phase, seq)
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	for _, task := range tasks {
		if err := os.WriteFile(filepath.Join(seqDir, task.name), []byte("# Task\n"), 0o644); err != nil {
			t.Fatalf("write task: %v", err)
		}
	}
	mgr, err := progress.NewManager(ctx, festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for _, task := range tasks {
		id := phase + "/" + seq + "/" + task.name
		switch task.state {
		case "completed":
			if err := mgr.MarkComplete(ctx, id); err != nil {
				t.Fatalf("MarkComplete(%s): %v", id, err)
			}
		case "in_progress":
			if err := mgr.MarkInProgress(ctx, id); err != nil {
				t.Fatalf("MarkInProgress(%s): %v", id, err)
			}
		case "blocked", "deferred":
			if err := mgr.ReportBlocker(ctx, id, task.reason, nil); err != nil {
				t.Fatalf("ReportBlocker(%s): %v", id, err)
			}
			if task.state == "deferred" {
				if err := mgr.DeferBlocker(ctx, id, task.deferReason, progress.DeferralAudit{Actor: "test"}); err != nil {
					t.Fatalf("DeferBlocker(%s): %v", id, err)
				}
			}
		}
	}
}

// setupGateBlockerFestival lays out an active festival whose implementation
// phase has a two-step GATES.md of judge-eligible checkpoints.
func setupGateBlockerFestival(t *testing.T) (festDir, phaseDir string) {
	t.Helper()
	festDir = t.TempDir()
	festYAML := "name: gate-blockers\nid: GB-001\nversion: \"1.0\"\nmetadata:\n  id: GB-001\n  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte(festYAML), 0o644); err != nil {
		t.Fatalf("write fest.yaml: %v", err)
	}
	phaseDir = filepath.Join(festDir, gateBlockerPhase)
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}
	goal := "---\nfest_type: phase\nfest_id: " + gateBlockerPhase + "\nfest_phase_type: implementation\nfest_status: pending\n---\n\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(goal), 0o644); err != nil {
		t.Fatalf("write PHASE_GOAL.md: %v", err)
	}
	gates := `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
---

# Implementation Phase Gate

## Step 1: COMPLETENESS — Verify Work Done

**Question:** Were all tasks completed?

**Actions:**
1. Verify task completion

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED

---

## Step 2: QUALITY — Verify Quality

**Question:** Do build and tests pass?

**Actions:**
1. Run build and tests

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED
`
	if err := os.WriteFile(filepath.Join(phaseDir, "GATES.md"), []byte(gates), 0o644); err != nil {
		t.Fatalf("write GATES.md: %v", err)
	}
	return festDir, phaseDir
}

// enterGatePhase isolates machine config and runs the workflow commands from
// inside the phase, the way an agent does.
func enterGatePhase(t *testing.T, festDir, phaseDir string) context.Context {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEST_CONFIG_DIR", t.TempDir())
	t.Setenv("PWD", phaseDir)
	t.Chdir(phaseDir)
	return scope.WithFestival(context.Background(), festDir)
}

// readProgressEvents returns every event line of the festival's progress log.
func readProgressEvents(t *testing.T, festDir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(festDir, progress.ProgressDir, progress.ProgressEventsFile))
	if err != nil {
		t.Fatalf("open progress events: %v", err)
	}
	defer func() { _ = f.Close() }()
	var events []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var e map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("decode event %q: %v", scanner.Text(), err)
		}
		events = append(events, e)
	}
	return events
}

func gateStepDone(events []map[string]any, step int) map[string]any {
	for _, e := range events {
		if e["event"] == "wf_step_done" && e["phase"] == "gate:"+gateBlockerPhase && e["step"] == float64(step) {
			return e
		}
	}
	return nil
}

// captureJudgeStdin installs a judge runner that records each request and
// returns verdict.
func captureJudgeStdin(t *testing.T, verdict string) *[][]byte {
	t.Helper()
	var requests [][]byte
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		requests = append(requests, append([]byte(nil), stdin...))
		return []byte(verdict), nil
	}))
	return &requests
}

const approveVerdict = `{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"evidence complete"}`

// The judge must see every blocked task in the phase, deferred or not, with
// the exact phase_tasks shape the ob judge renders. A WORKFLOW.md checkpoint
// is never refused, so the request carries both kinds.
func TestJudgeRequestCarriesPhaseTasks(t *testing.T) {
	festDir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(festDir, "001_INGEST")
	seedPhaseTasks(t, festDir, "001_INGEST", "01_build", []seededTask{
		{name: "01_task.md", state: "completed"},
		{name: "02_task.md", state: "blocked", reason: "vendor API removed"},
		{name: "03_task.md", state: "deferred", reason: "waiting on signing key", deferReason: "release can ship unsigned first"},
		{name: "04_task.md", state: "in_progress"},
		{name: "05_task.md", state: "pending"},
	})

	nav := getNavigator(t, phaseDir)
	ctx := context.Background()
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	requests := captureJudgeStdin(t, approveVerdict)

	if _, _, err := judgeApproval(ctx, nav, nav.GetSteps()[1], approvalJudgeOptions{JudgeCommand: "fake judge"}); err != nil {
		t.Fatalf("judgeApproval: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("judge ran %d times, want 1", len(*requests))
	}
	var req struct {
		PhaseTasks json.RawMessage `json:"phase_tasks"`
	}
	if err := json.Unmarshal((*requests)[0], &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(req.PhaseTasks) == 0 {
		t.Fatalf("judge request has no phase_tasks: %s", (*requests)[0])
	}
	want := `{"total":5,"completed":1,"skipped":0,"in_progress":1,"pending":1,"blocked":[` +
		`{"task":"01_build/02_task.md","reason":"vendor API removed","deferred":false,"deferred_reason":""},` +
		`{"task":"01_build/03_task.md","reason":"waiting on signing key","deferred":true,"deferred_reason":"release can ship unsigned first"}]}`
	if string(req.PhaseTasks) != want {
		t.Fatalf("phase_tasks =\n%s\nwant\n%s", req.PhaseTasks, want)
	}
}

// A gate is judged with its deferred blockers in view, and a task blocked
// without a deferral after the gate was reached stops the judge before it runs.
func TestGateJudgeSeesDeferredBlockersAndRefusesOpenOnes(t *testing.T) {
	festDir, phaseDir := setupGateBlockerFestival(t)
	seedPhaseTasks(t, festDir, gateBlockerPhase, "01_build", []seededTask{
		{name: "01_task.md", state: "completed"},
		{name: "02_task.md", state: "deferred", reason: "vendor API removed", deferReason: "independent of the gate"},
	})
	ctx := enterGatePhase(t, festDir, phaseDir)

	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator: %v", err)
	}
	if !nav.IsGate() {
		t.Fatal("navigator should target GATES.md once every task is settled")
	}
	requests := captureJudgeStdin(t, approveVerdict)
	step := nav.GetSteps()[0]

	if _, _, err := judgeApproval(ctx, nav, step, approvalJudgeOptions{JudgeCommand: "fake judge"}); err != nil {
		t.Fatalf("judgeApproval with only deferred blockers: %v", err)
	}
	var req struct {
		Document   string `json:"document"`
		PhaseTasks *struct {
			Total   int `json:"total"`
			Blocked []struct {
				Task     string `json:"task"`
				Deferred bool   `json:"deferred"`
			} `json:"blocked"`
		} `json:"phase_tasks"`
	}
	if err := json.Unmarshal((*requests)[0], &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if req.Document != "GATES.md" || req.PhaseTasks == nil || req.PhaseTasks.Total != 2 ||
		len(req.PhaseTasks.Blocked) != 1 || req.PhaseTasks.Blocked[0].Task != "01_build/02_task.md" ||
		!req.PhaseTasks.Blocked[0].Deferred {
		t.Fatalf("gate request must carry the deferred blocker, got %s", (*requests)[0])
	}

	// The blocker comes back without a deferral after the navigator loaded.
	mgr, err := progress.NewManager(context.Background(), festDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	taskID := gateBlockerPhase + "/01_build/02_task.md"
	if err := mgr.ClearBlocker(context.Background(), taskID, "retry"); err != nil {
		t.Fatalf("ClearBlocker: %v", err)
	}
	if err := mgr.ReportBlocker(context.Background(), taskID, "vendor API still removed", nil); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}

	_, _, err = judgeApproval(ctx, nav, step, approvalJudgeOptions{JudgeCommand: "fake judge"})
	if len(*requests) != 1 {
		t.Fatalf("judge ran with a blocker that is not deferred (%d runs)", len(*requests))
	}
	assertOpenBlockerRefusal(t, err, "01_build/02_task.md", "vendor API still removed")
}

func assertOpenBlockerRefusal(t *testing.T, err error, task, reason string) {
	t.Helper()
	if err == nil {
		t.Fatal("gate must be refused while a blocked task is not deferred")
	}
	msg := err.Error()
	for _, want := range []string{task, reason, "fest task defer", "not deferred"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal must name %q, got:\n%s", want, msg)
		}
	}
}

// advance, approve, and the judge all refuse a gate while a task in the phase
// is blocked without a deferral, and record nothing.
func TestGateCommandsRefuseOpenBlocker(t *testing.T) {
	festDir, phaseDir := setupGateBlockerFestival(t)
	seedPhaseTasks(t, festDir, gateBlockerPhase, "01_build", []seededTask{
		{name: "01_task.md", state: "completed"},
		{name: "02_task.md", state: "blocked", reason: "vendor API removed"},
	})
	ctx := enterGatePhase(t, festDir, phaseDir)
	requests := captureJudgeStdin(t, approveVerdict)
	before := len(readProgressEvents(t, festDir))

	for name, run := range map[string]func() error{
		"advance": func() error { return runAdvance(ctx) },
		"approve": func() error { return runApprove(ctx) },
		"judge": func() error {
			return runApproveWithOptions(ctx, wf.DecisionMetadata{}, approvalJudgeOptions{Auto: true, Rejudge: true, Wait: true})
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertOpenBlockerRefusal(t, run(), "01_build/02_task.md", "vendor API removed")
		})
	}
	if len(*requests) != 0 {
		t.Fatalf("judge ran %d times with an open blocker", len(*requests))
	}
	if after := len(readProgressEvents(t, festDir)); after != before {
		t.Fatalf("refused gate commands wrote %d progress events", after-before)
	}
}

// With only deferred blockers the gate proceeds, and both approval routes
// record how many blockers were deferred on the step's approval event.
func TestGateApprovalRecordsDeferredBlockerCount(t *testing.T) {
	festDir, phaseDir := setupGateBlockerFestival(t)
	seedPhaseTasks(t, festDir, gateBlockerPhase, "01_build", []seededTask{
		{name: "01_task.md", state: "completed"},
		{name: "02_task.md", state: "deferred", reason: "vendor API removed", deferReason: "independent of the gate"},
		{name: "03_task.md", state: "deferred", reason: "signing key pending", deferReason: "ships unsigned first"},
	})
	ctx := enterGatePhase(t, festDir, phaseDir)

	// Operator approval of step 1.
	captureStdout(t, func() {
		if err := runApprove(ctx); err != nil {
			t.Fatalf("approve with only deferred blockers: %v", err)
		}
	})

	// Judge approval of step 2.
	nav, err := getWorkflowNavigator(ctx)
	if err != nil {
		t.Fatalf("getWorkflowNavigator: %v", err)
	}
	if got := nav.GetWorkflowState().CurrentStep; got != 2 {
		t.Fatalf("current gate step = %d, want 2", got)
	}
	captureJudgeStdin(t, approveVerdict)
	captureStdout(t, func() {
		if err := runApproveAuto(ctx, nav, 2, nav.GetSteps()[1], approvalJudgeOptions{
			JudgeCommand: "fake judge", Timeout: time.Second, Wait: true,
		}); err != nil {
			t.Fatalf("runApproveAuto: %v", err)
		}
	})

	events := readProgressEvents(t, festDir)
	for step := 1; step <= 2; step++ {
		done := gateStepDone(events, step)
		if done == nil {
			t.Fatalf("no wf_step_done for gate step %d", step)
		}
		if got := done["deferred_blockers"]; got != float64(2) {
			t.Fatalf("gate step %d approval event deferred_blockers = %v, want 2: %v", step, got, done)
		}
	}

	store := progress.NewStore(festDir)
	if err := store.LoadReadOnly(context.Background()); err != nil {
		t.Fatalf("LoadReadOnly: %v", err)
	}
	gate, ok := store.GatePhaseState(gateBlockerPhase)
	if !ok || !gate.IsComplete() {
		t.Fatalf("gate should be complete, got %+v", gate)
	}
	raw, err := json.Marshal(gate.GetStepState(2))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"deferred_blockers":2`) {
		t.Fatalf("replayed gate step 2 lost the deferred blocker count: %s", raw)
	}
}

// fest workflow status lists the phase's blocked tasks with the gate.
func TestWorkflowStatusListsGateBlockers(t *testing.T) {
	festDir, phaseDir := setupGateBlockerFestival(t)
	seedPhaseTasks(t, festDir, gateBlockerPhase, "01_build", []seededTask{
		{name: "01_task.md", state: "completed"},
		{name: "02_task.md", state: "deferred", reason: "vendor API removed", deferReason: "independent of the gate"},
	})
	ctx := enterGatePhase(t, festDir, phaseDir)

	out := captureStdout(t, func() {
		if err := runStatus(ctx, false); err != nil {
			t.Fatalf("runStatus: %v", err)
		}
	})
	for _, want := range []string{
		"BLOCKED TASKS IN THIS PHASE: 1 (1 deferred)",
		"01_build/02_task.md [deferred]",
		"Blocker: vendor API removed",
		"Deferred because: independent of the gate",
		"must account for them",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("workflow status for the gate lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "BLOCKED TASKS") > strings.Index(out, "Steps:") {
		t.Fatalf("blocked tasks must be listed before the gate steps:\n%s", out)
	}
}
