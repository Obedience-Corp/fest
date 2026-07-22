package hooks

import (
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

func TestPlanBindings_Undeclared(t *testing.T) {
	eff, err := Resolve(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := eff.PlanBindings(LevelTask, []string{"missing"}, nil)
	if len(plan) != 1 || plan[0].Skip != SkipUndeclared {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanBindings_Disabled(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{
		Levels: map[string]bool{"task": false},
		Definitions: map[string]config.HookDefinition{
			"a": {Command: "true"},
			"b": {Command: "true", Enabled: bp(false)},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := eff.PlanBindings(LevelTask, []string{"a", "b"}, nil)
	if len(plan) != 2 {
		t.Fatalf("plan len = %d", len(plan))
	}
	if plan[0].Skip != SkipDisabled || plan[1].Skip != SkipDisabled {
		t.Fatalf("want both disabled: %+v", plan)
	}
}

func TestPlanBindings_RunnableAndOrder(t *testing.T) {
	eff, err := Resolve(nil, &config.HooksConfig{
		Definitions: map[string]config.HookDefinition{
			"pre1":  {Command: "true"},
			"post1": {Command: "true"},
			"post2": {Command: "true"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := eff.PlanBindings(LevelTask, []string{"pre1"}, []string{"post1", "missing", "post2"})
	if len(plan) != 4 {
		t.Fatalf("plan = %+v", plan)
	}
	want := []struct {
		name string
		t    Timing
		skip SkipReason
	}{
		{"pre1", TimingPre, ""},
		{"post1", TimingPost, ""},
		{"missing", TimingPost, SkipUndeclared},
		{"post2", TimingPost, ""},
	}
	for i, w := range want {
		if plan[i].Name != w.name || plan[i].Timing != w.t || plan[i].Skip != w.skip {
			t.Fatalf("plan[%d]=%+v want name=%s timing=%s skip=%s", i, plan[i], w.name, w.t, w.skip)
		}
		if w.skip == "" && plan[i].Hook.Command == "" {
			t.Fatalf("runnable plan[%d] missing Hook", i)
		}
	}
	line := FormatSkippedUndeclaredLine(plan)
	if line != "Skipped hooks (undeclared): missing" {
		t.Fatalf("line = %q", line)
	}
}

func TestPlanBindings_Empty(t *testing.T) {
	eff, err := Resolve(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan := eff.PlanBindings(LevelGate, nil, nil); len(plan) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestV1Verbs_ClosedSet(t *testing.T) {
	verbs := V1Verbs()
	if len(verbs) != 4 {
		t.Fatalf("want exactly 4 verbs, got %v", verbs)
	}
	seen := map[Verb]bool{}
	for _, v := range verbs {
		seen[v] = true
	}
	for _, need := range []Verb{VerbTaskComplete, VerbSequenceComplete, VerbPhaseComplete, VerbGateApprove} {
		if !seen[need] {
			t.Fatalf("missing verb %s", need)
		}
	}
}
