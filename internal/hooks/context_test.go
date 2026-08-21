package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPayload_JSONAndEnv(t *testing.T) {
	p := BuildPayload(Coord{
		FestivalPath: "/campaigns/demo/festivals/active/demo-AB0001",
		FestivalID:   "AB0001",
		Phase:        "001_IMPLEMENT",
		Step:         2,
		Task:         "001_IMPLEMENT/01_seq/03_task.md",
	}, LevelTask, VerbTaskComplete, PlannedHook{Name: "buzz_status", Timing: TimingPost})

	raw := p.JSON()
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("JSON must end with newline: %q", raw)
	}
	var got Payload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SchemaVersion != ContextSchemaVersion {
		t.Fatalf("schema = %q", got.SchemaVersion)
	}
	if got.Task != "001_IMPLEMENT/01_seq/03_task.md" || got.Verb != string(VerbTaskComplete) {
		t.Fatalf("payload = %+v", got)
	}
	if got.Hook != "buzz_status" || got.Timing != string(TimingPost) {
		t.Fatalf("hook/timing = %+v", got)
	}

	env := strings.Join(p.Env(), "\n")
	for _, want := range []string{
		"FEST_HOOK=1",
		"FEST_HOOK_SCHEMA=" + ContextSchemaVersion,
		"FEST_HOOK_NAME=buzz_status",
		"FEST_TASK=001_IMPLEMENT/01_seq/03_task.md",
		"FEST_VERB=task_complete",
		"FEST_LEVEL=task",
		"FEST_TIMING=post",
		"FEST_PHASE=001_IMPLEMENT",
		"FEST_STEP=2",
		"FEST_FESTIVAL_PATH=/campaigns/demo/festivals/active/demo-AB0001",
		"FEST_FESTIVAL=AB0001",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %q:\n%s", want, env)
		}
	}
}

func TestBuildPayload_OmitsEmptyOptionalFields(t *testing.T) {
	p := BuildPayload(Coord{}, LevelGate, VerbGateApprove, PlannedHook{Name: "lint", Timing: TimingPre})
	raw := string(p.JSON())
	for _, banned := range []string{`"task"`, `"phase"`, `"festival_id"`, `"festival_path"`, `"step"`} {
		if strings.Contains(raw, banned) {
			t.Fatalf("empty field %s should be omitted: %s", banned, raw)
		}
	}
	env := strings.Join(p.Env(), "\n")
	if strings.Contains(env, "FEST_TASK=") || strings.Contains(env, "FEST_STEP=") {
		t.Fatalf("empty task/step must not appear in env:\n%s", env)
	}
	if !strings.Contains(env, "FEST_HOOK=1") || !strings.Contains(env, "FEST_VERB=gate_approve") {
		t.Fatalf("required env missing:\n%s", env)
	}
}

func TestFestivalID_ReadsMetadata(t *testing.T) {
	dir := t.TempDir()
	body := []byte("version: \"1.0\"\nmetadata:\n  id: XY0009\n  name: Demo\n")
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FestivalID(dir); got != "XY0009" {
		t.Fatalf("FestivalID = %q, want XY0009", got)
	}
	if got := FestivalID(""); got != "" {
		t.Fatalf("empty path = %q", got)
	}
}
