package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/progress"
)

func decodeStatusJSON(t *testing.T, nav *wf.Navigator) workflowStatusJSON {
	t.Helper()
	out, err := renderWorkflowStatusJSON(context.Background(), nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	var got workflowStatusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal status json: %v\noutput: %s", err, out)
	}
	return got
}

func TestWorkflowStatusJSON_Normal(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)

	got := decodeStatusJSON(t, nav)

	if got.SchemaVersion != workflowStatusSchema {
		t.Errorf("schema_version = %q, want %q", got.SchemaVersion, workflowStatusSchema)
	}
	if got.FestivalID != "TEST-001" {
		t.Errorf("festival_id = %q, want TEST-001", got.FestivalID)
	}
	if got.FestivalPath != dir {
		t.Errorf("festival_path = %q, want %q", got.FestivalPath, dir)
	}
	if got.Phase != "001_INGEST" {
		t.Errorf("phase = %q, want 001_INGEST", got.Phase)
	}
	if got.PhasePath != phaseDir {
		t.Errorf("phase_path = %q, want %q", got.PhasePath, phaseDir)
	}
	if got.Mode != "workflow" {
		t.Errorf("mode = %q, want workflow", got.Mode)
	}
	if got.Workflow != "001_INGEST" {
		t.Errorf("workflow = %q, want 001_INGEST", got.Workflow)
	}
	if got.Complete {
		t.Error("complete = true, want false for fresh workflow")
	}
	if got.TotalSteps != 3 {
		t.Errorf("total_steps = %d, want 3", got.TotalSteps)
	}
	if got.CurrentStep == nil || *got.CurrentStep != 1 {
		t.Errorf("current_step = %v, want 1", got.CurrentStep)
	}
	if got.WorkflowStep == nil || *got.WorkflowStep != "READ" {
		t.Errorf("workflow_step = %v, want READ", got.WorkflowStep)
	}
	if len(got.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(got.Steps))
	}
	if !got.Steps[0].IsCurrent {
		t.Error("step 1 is_current = false, want true")
	}
	if got.Steps[0].Name != "READ" {
		t.Errorf("step 1 name = %q, want READ", got.Steps[0].Name)
	}
	if got.Steps[0].HasCheckpoint {
		t.Error("step 1 has_checkpoint = true, want false")
	}
	// Step 2 (ANALYZE) declares USER APPROVAL REQUIRED.
	if !got.Steps[1].HasCheckpoint {
		t.Error("step 2 has_checkpoint = false, want true")
	}
}

func TestWorkflowStatusJSON_Complete(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	ctx := context.Background()

	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 1: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("advance step 2: %v", err)
	}
	if err := nav.Approve(ctx); err != nil {
		t.Fatalf("approve checkpoint: %v", err)
	}
	if err := nav.Advance(ctx); err != nil && err != guidance.ErrAlreadyComplete {
		if !nav.GetWorkflowState().IsComplete() {
			t.Fatalf("advance step 3: %v", err)
		}
	}
	if !nav.GetWorkflowState().IsComplete() {
		t.Fatal("workflow should be complete before asserting JSON")
	}

	got := decodeStatusJSON(t, nav)

	if !got.Complete {
		t.Error("complete = false, want true")
	}
	if got.CurrentStep != nil {
		t.Errorf("current_step = %v, want null when complete", *got.CurrentStep)
	}
	if got.WorkflowStep != nil {
		t.Errorf("workflow_step = %v, want null when complete", *got.WorkflowStep)
	}
	for i, s := range got.Steps {
		if s.IsCurrent {
			t.Errorf("step %d is_current = true, want false when complete", i+1)
		}
	}
}

func TestWorkflowStatusJSON_GateMode(t *testing.T) {
	dir := setupGateOnlyFestival(t)
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator: %v", err)
	}

	got := decodeStatusJSON(t, nav)

	if got.Mode != "gate" {
		t.Errorf("mode = %q, want gate", got.Mode)
	}
	if got.Phase != "001_IMPLEMENT" {
		t.Errorf("phase = %q, want 001_IMPLEMENT", got.Phase)
	}
	if got.TotalSteps != 2 {
		t.Errorf("total_steps = %d, want 2", got.TotalSteps)
	}
}

func TestWorkflowStatusJSON_EmptyStepsSerializesArray(t *testing.T) {
	dir := t.TempDir()
	phaseDir := filepath.Join(dir, "001_EMPTY")
	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatalf("mkdir phase: %v", err)
	}
	nav := getNavigator(t, phaseDir) // no WORKFLOW.md/GATES.md => empty steps

	out, err := renderWorkflowStatusJSON(context.Background(), nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	// Empty steps must serialize as [] (not null) for stable consumers.
	if !strings.Contains(out, `"steps": []`) {
		t.Errorf("expected empty steps array, got:\n%s", out)
	}

	got := decodeStatusJSON(t, nav)
	if !got.Complete {
		t.Error("complete = false, want true for empty workflow")
	}
	if got.CurrentStep != nil {
		t.Errorf("current_step = %v, want null for empty workflow", *got.CurrentStep)
	}
	if got.WorkflowStep != nil {
		t.Error("workflow_step should be null for empty workflow")
	}
	if len(got.Steps) != 0 {
		t.Errorf("steps len = %d, want 0", len(got.Steps))
	}
}

func TestWorkflowStatusJSON_ConciseJudgeFeedback(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav := getNavigator(t, phaseDir)
	if err := nav.Advance(context.Background()); err != nil {
		t.Fatalf("advance: %v", err)
	}
	step := nav.GetWorkflowState().GetStepState(2)
	step.Status = wf.StepStatusBlocked
	step.Feedback = `approval auto mode: schema_version=fest.approval.judge/v1 judge_command="ob judge" decision=reject reason="missing acceptance proof"`
	step.Judge = &wf.JudgeState{Status: wf.JudgeRejected, Command: "ob judge", Detail: "missing acceptance proof", Pid: 42, RunID: "run-1"}

	out, err := renderWorkflowStatusJSON(context.Background(), nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	if !strings.Contains(out, `"feedback": "missing acceptance proof"`) {
		t.Fatalf("JSON missing concise feedback:\n%s", out)
	}
	for _, want := range []string{`"judge_command": "ob judge"`, `"judge_pid": 42`, `"judge_run_id": "run-1"`, `"judge_detail": "missing acceptance proof"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON missing v1 compatibility field %q:\n%s", want, out)
		}
	}
}

// TestRunStatus_JSONAndText exercises the real command entrypoint for both
// modes: --json emits parseable structured output and default text mode still
// renders the human-readable status without leaking JSON.
func TestRunStatus_JSONAndText(t *testing.T) {
	dir := setupWorkflowFestival(t)
	phaseDir := filepath.Join(dir, "001_INGEST")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	ctx := context.Background()

	jsonOut := captureStdout(t, func() {
		if err := runStatus(ctx, true); err != nil {
			t.Fatalf("runStatus(json): %v", err)
		}
	})
	var parsed workflowStatusJSON
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("json mode did not emit valid JSON: %v\noutput: %s", err, jsonOut)
	}
	if parsed.SchemaVersion != workflowStatusSchema {
		t.Errorf("schema_version = %q, want %q", parsed.SchemaVersion, workflowStatusSchema)
	}

	textOut := captureStdout(t, func() {
		if err := runStatus(ctx, false); err != nil {
			t.Fatalf("runStatus(text): %v", err)
		}
	})
	if !strings.Contains(textOut, "Workflow Status") {
		t.Errorf("text mode missing human header, got:\n%s", textOut)
	}
	if strings.Contains(textOut, "schema_version") {
		t.Errorf("text mode leaked JSON: %s", textOut)
	}
}

// judgeRejectedNavigator drives a real judge rejection through the navigator so
// the status snapshot is asserted against durable state, not a hand-built one.
func judgeRejectedNavigator(t *testing.T, dir string, extras wf.JudgeVerdictExtras, followups []string) *wf.Navigator {
	t.Helper()

	ctx := context.Background()
	phaseDir := filepath.Join(dir, "001_INGEST")
	nav, err := wf.NewNavigator(createGuidanceContext(phaseDir), guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}
	store := progress.NewStore(dir)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	nav.SetStateStore(store)
	if err := nav.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := nav.Advance(ctx); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if err := nav.BeginJudge(ctx, 2, "ob judge", "run-1", 4242, wf.JudgeInputs{}); err != nil {
		t.Fatalf("BeginJudge: %v", err)
	}
	decision := wf.DecisionMetadata{Actor: decisionActorAgent, Summary: "missing acceptance proof", Followups: followups}
	applied, err := nav.ApplyJudgeRejection(ctx, 2, "run-1", "audit", decision, extras)
	if err != nil || !applied {
		t.Fatalf("ApplyJudgeRejection: applied=%v err=%v", applied, err)
	}

	reloaded, err := reloadWorkflowNavigator(ctx, nav)
	if err != nil {
		t.Fatalf("reloadWorkflowNavigator: %v", err)
	}
	return reloaded
}

func TestWorkflowStatusJSON_FollowupsAndCompleteVerdictSurviveReload(t *testing.T) {
	confidence := 0.8
	followups := []string{"attach the gate run output", "link the failing test"}
	nav := judgeRejectedNavigator(t, setupWorkflowFestival(t),
		wf.JudgeVerdictExtras{Confidence: &confidence, EvidenceStatus: "ok"}, followups)

	step := decodeStatusJSON(t, nav).Steps[1]

	if len(step.Followups) != len(followups) {
		t.Fatalf("followups = %#v, want %#v", step.Followups, followups)
	}
	for i, want := range followups {
		if step.Followups[i] != want {
			t.Errorf("followups[%d] = %q, want %q", i, step.Followups[i], want)
		}
	}
	if step.JudgeStatus != wf.JudgeRejected {
		t.Errorf("judge_status = %q, want %q", step.JudgeStatus, wf.JudgeRejected)
	}
	if step.JudgeConfidence == nil || *step.JudgeConfidence != confidence {
		t.Errorf("judge_confidence = %v, want %v", step.JudgeConfidence, confidence)
	}
	if step.JudgeEvidenceStatus != "ok" {
		t.Errorf("judge_evidence_status = %q, want ok", step.JudgeEvidenceStatus)
	}
	if step.JudgeFinishedAt == "" {
		t.Error("judge_finished_at is empty, want the recorded verdict time")
	} else if _, err := time.Parse(time.RFC3339, step.JudgeFinishedAt); err != nil {
		t.Errorf("judge_finished_at = %q, want RFC 3339: %v", step.JudgeFinishedAt, err)
	}
}

func TestWorkflowStatusJSON_JudgeWithoutExtrasOmitsThem(t *testing.T) {
	nav := judgeRejectedNavigator(t, setupWorkflowFestival(t), wf.JudgeVerdictExtras{}, nil)

	out, err := renderWorkflowStatusJSON(context.Background(), nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	for _, absent := range []string{"judge_confidence", "judge_evidence_status", "recent_hook_runs"} {
		if strings.Contains(out, absent) {
			t.Errorf("%q present for a judge that reported none and a festival with no hook runs:\n%s", absent, out)
		}
	}
	if !strings.Contains(out, `"judge_finished_at"`) {
		t.Errorf("judge_finished_at missing from a recorded verdict:\n%s", out)
	}
}

// preChangeStatusStepJSON is the fest.workflow.status/v1 step shape as it stood
// before followups, the full verdict, and hook runs were added. A consumer
// written against it must keep parsing the new output unchanged.
type preChangeStatusStepJSON struct {
	Number                int    `json:"number"`
	Name                  string `json:"name"`
	Status                string `json:"status"`
	IsCurrent             bool   `json:"is_current"`
	HasCheckpoint         bool   `json:"has_checkpoint"`
	Goal                  string `json:"goal"`
	Feedback              string `json:"feedback,omitempty"`
	RemediationPhase      string `json:"remediation_phase,omitempty"`
	WaitingOnJudge        bool   `json:"waiting_on_judge,omitempty"`
	JudgeStatus           string `json:"judge_status,omitempty"`
	JudgeCommand          string `json:"judge_command,omitempty"`
	JudgePid              int    `json:"judge_pid,omitempty"`
	JudgeRunID            string `json:"judge_run_id,omitempty"`
	JudgeDetail           string `json:"judge_detail,omitempty"`
	HumanApprovalRequired bool   `json:"human_approval_required,omitempty"`
}

type preChangeStatusJSON struct {
	SchemaVersion string                    `json:"schema_version"`
	FestivalID    string                    `json:"festival_id"`
	FestivalName  string                    `json:"festival_name"`
	FestivalPath  string                    `json:"festival_path"`
	Phase         string                    `json:"phase"`
	PhasePath     string                    `json:"phase_path"`
	Mode          string                    `json:"mode"`
	Workflow      string                    `json:"workflow"`
	WorkflowStep  *string                   `json:"workflow_step"`
	CurrentStep   *int                      `json:"current_step"`
	TotalSteps    int                       `json:"total_steps"`
	Complete      bool                      `json:"complete"`
	Steps         []preChangeStatusStepJSON `json:"steps"`
}

func TestWorkflowStatusJSON_OldConsumerStillParses(t *testing.T) {
	confidence := 0.8
	nav := judgeRejectedNavigator(t, setupWorkflowFestival(t),
		wf.JudgeVerdictExtras{Confidence: &confidence, EvidenceStatus: "ok"}, []string{"attach the gate run output"})

	out, err := renderWorkflowStatusJSON(context.Background(), nav)
	if err != nil {
		t.Fatalf("renderWorkflowStatusJSON: %v", err)
	}
	var old preChangeStatusJSON
	if err := json.Unmarshal([]byte(out), &old); err != nil {
		t.Fatalf("pre-change consumer failed to parse new output: %v\n%s", err, out)
	}

	if old.SchemaVersion != "fest.workflow.status/v1" {
		t.Errorf("schema_version = %q, want fest.workflow.status/v1 (the change is additive)", old.SchemaVersion)
	}
	if old.FestivalID != "TEST-001" || old.Phase != "001_INGEST" || old.TotalSteps != 3 {
		t.Errorf("header fields lost: %+v", old)
	}
	if len(old.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(old.Steps))
	}
	step := old.Steps[1]
	if step.Number != 2 || step.Name != "ANALYZE" || step.Status != string(wf.StepStatusBlocked) {
		t.Errorf("step identity fields lost: %+v", step)
	}
	if step.JudgeStatus != wf.JudgeRejected || step.JudgeCommand != "ob judge" ||
		step.JudgePid != 4242 || step.JudgeRunID != "run-1" || step.JudgeDetail == "" {
		t.Errorf("judge fields lost for a pre-change consumer: %+v", step)
	}
	if !step.HasCheckpoint || step.Feedback == "" {
		t.Errorf("checkpoint fields lost for a pre-change consumer: %+v", step)
	}
}

// writeHookRuns appends wf_hook_run events under an explicit state key. The key
// is spelled out by the caller so a gate test proves the reader matches the
// prefixed key the emitters use, not the bare phase name.
func writeHookRuns(t *testing.T, festivalPath, stateKey string, step int, names ...string) {
	t.Helper()

	ctx := context.Background()
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	for _, name := range names {
		progress.QueueHookRuns(store, stateKey, step, []hooks.HookRun{{
			Name:     name,
			Layer:    hooks.LayerFestivals,
			Timing:   hooks.TimingPost,
			Verb:     hooks.VerbGateApprove,
			Outcome:  hooks.OutcomePass,
			Duration: 25 * time.Second,
			Fail:     hooks.FailClosed,
		}})
	}
	if err := store.Save(ctx); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
}

func TestWorkflowStatusJSON_HookRunsAreGroupedByStepAndBounded(t *testing.T) {
	dir := setupWorkflowFestival(t)
	overflow := make([]string, 0, progress.MaxRecentHookRuns+3)
	for i := range progress.MaxRecentHookRuns + 3 {
		overflow = append(overflow, "step1-hook-"+strconv.Itoa(i))
	}
	writeHookRuns(t, dir, "001_INGEST", 1, overflow...)
	writeHookRuns(t, dir, "001_INGEST", 2, "step2-hook")

	nav := getNavigator(t, filepath.Join(dir, "001_INGEST"))
	got := decodeStatusJSON(t, nav)

	// The cap is on the phase read, so step 1 keeps only what survives it.
	step1 := got.Steps[0].RecentHookRuns
	step2 := got.Steps[1].RecentHookRuns
	if len(step1)+len(step2) != progress.MaxRecentHookRuns {
		t.Fatalf("total runs = %d, want the cap of %d", len(step1)+len(step2), progress.MaxRecentHookRuns)
	}
	if len(step2) != 1 || step2[0].Name != "step2-hook" {
		t.Fatalf("step 2 runs = %+v, want only its own newest run", step2)
	}
	for _, run := range step1 {
		if !strings.HasPrefix(run.Name, "step1-hook-") {
			t.Errorf("step 1 carries another step's run: %+v", run)
		}
	}
	oldestKept := "step1-hook-" + strconv.Itoa(len(overflow)-len(step1))
	newestKept := "step1-hook-" + strconv.Itoa(len(overflow)-1)
	if step1[0].Name != oldestKept || step1[len(step1)-1].Name != newestKept {
		t.Errorf("step 1 runs = %+v, want %s..%s oldest first", step1, oldestKept, newestKept)
	}
	if len(got.Steps[2].RecentHookRuns) != 0 {
		t.Errorf("step 3 runs = %+v, want none", got.Steps[2].RecentHookRuns)
	}
	newest := step2[0]
	if newest.Layer != "festivals" || newest.Timing != "post" || newest.Verb != "gate_approve" ||
		newest.Outcome != "pass" || newest.Fail != "closed" || newest.DurationMS != 25000 {
		t.Errorf("hook run fields = %+v", newest)
	}
	if _, err := time.Parse(time.RFC3339, newest.At); err != nil {
		t.Errorf("hook run at = %q, want RFC 3339: %v", newest.At, err)
	}
}

func TestWorkflowStatusJSON_GateHookRunsMatchThePrefixedStateKey(t *testing.T) {
	dir := setupGateOnlyFestival(t)
	phaseDir := filepath.Join(dir, "001_IMPLEMENT")
	writeHookRuns(t, dir, "gate:001_IMPLEMENT", 1, "approval_judge")
	writeHookRuns(t, dir, "001_IMPLEMENT", 1, "wrong_key_hook")

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(phaseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	nav, err := getWorkflowNavigator(context.Background())
	if err != nil {
		t.Fatalf("getWorkflowNavigator: %v", err)
	}
	if nav.StateKey() != "gate:001_IMPLEMENT" {
		t.Fatalf("StateKey() = %q, want gate:001_IMPLEMENT", nav.StateKey())
	}

	runs := decodeStatusJSON(t, nav).Steps[0].RecentHookRuns
	if len(runs) != 1 || runs[0].Name != "approval_judge" {
		t.Fatalf("gate runs = %+v, want only the run written under the prefixed key", runs)
	}
}
