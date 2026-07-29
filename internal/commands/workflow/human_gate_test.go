package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func TestIsHumanRequired(t *testing.T) {
	if !isHumanRequired(wf.WorkflowStep{Approval: "human-required"}) {
		t.Fatal("expected true")
	}
	if isHumanRequired(wf.WorkflowStep{Approval: ""}) {
		t.Fatal("expected false")
	}
}

func TestHumanRequiredAutoRefusalNamesGate(t *testing.T) {
	err := humanRequiredAutoRefusal(3, wf.WorkflowStep{Name: "SHIP"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "human-required") && !strings.Contains(msg, "cannot auto-clear") {
		t.Fatalf("msg = %q", msg)
	}
	if !strings.Contains(msg, "SHIP") {
		t.Fatalf("gate name missing: %q", msg)
	}
}

func TestRequireHumanGateTTY_NonTTY(t *testing.T) {
	orig := workflowApproveTTYCheck
	t.Cleanup(func() { workflowApproveTTYCheck = orig })
	workflowApproveTTYCheck = func(fd int) bool { return false }
	if err := requireHumanGateTTY(); err == nil {
		t.Fatal("expected TTY error")
	}
}

func TestRequireHumanGateTTY_TTY(t *testing.T) {
	orig := workflowApproveTTYCheck
	t.Cleanup(func() { workflowApproveTTYCheck = orig })
	workflowApproveTTYCheck = func(fd int) bool { return true }
	if err := requireHumanGateTTY(); err != nil {
		t.Fatal(err)
	}
}

func TestHumanApprovalRequiredStatusJSONKey(t *testing.T) {
	raw, err := json.Marshal(workflowStatusStepJSON{HumanApprovalRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"human_approval_required":true`) {
		t.Fatalf("json key missing: %s", raw)
	}
	raw, err = json.Marshal(workflowStatusStepJSON{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "human_approval_required") {
		t.Fatalf("field must be omitempty when false: %s", raw)
	}
}
