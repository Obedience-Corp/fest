package progress

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

func TestRunLifecycleStage_PassesTaskContextStdin(t *testing.T) {
	var got []byte
	runner := hooks.NewRunner("")
	runner.Exec = func(ctx context.Context, command string, stdin []byte, dir string) hooks.CommandResult {
		got = append([]byte(nil), stdin...)
		return hooks.CommandResult{ExitCode: 0}
	}
	req := LifecycleHookRequest{
		FestivalPath: "/fest",
		FestivalID:   "AB0001",
		Phase:        "001_PHASE",
		Task:         "001_PHASE/01_seq/01_task.md",
		Level:        hooks.LevelTask,
		Verb:         hooks.VerbTaskComplete,
	}
	planned := []hooks.PlannedHook{{
		Name: "buzz_status", Timing: hooks.TimingPost,
		Hook: hooks.ResolvedHook{Command: "camp-buzz"},
	}}
	if _, _, err := RunLifecycleStage(context.Background(), nil, runner, req, planned, hooks.TimingPost); err != nil {
		t.Fatal(err)
	}
	var payload hooks.Payload
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("stdin is not context JSON: %v (%q)", err, got)
	}
	if payload.SchemaVersion != hooks.ContextSchemaVersion {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.Task != req.Task || payload.FestivalID != "AB0001" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Verb != string(hooks.VerbTaskComplete) || payload.Timing != string(hooks.TimingPost) {
		t.Fatalf("verb/timing = %+v", payload)
	}
	if runner.Coord.Task != req.Task {
		t.Fatalf("runner.Coord = %+v", runner.Coord)
	}
}

func TestLifecycleHookRequest_CoordLoadsFestivalID(t *testing.T) {
	req := LifecycleHookRequest{FestivalID: "KEEP-ME", FestivalPath: "/unused"}
	if got := req.Coord().FestivalID; got != "KEEP-ME" {
		t.Fatalf("explicit id overwritten: %q", got)
	}
}
