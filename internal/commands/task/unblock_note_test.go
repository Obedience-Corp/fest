package task

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func unblockJSONResult(t *testing.T, festDir, taskRel string) map[string]any {
	t.Helper()
	unblockJSON = true
	t.Cleanup(func() { unblockJSON, unblockNote = false, "" })

	output := captureIO(t, func() {
		if err := runUnblock(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runUnblock() error = %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decoding unblock JSON %q: %v", output, err)
	}
	return result
}

func TestUnblockJSONIsUnchangedWithoutANote(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)
	unblockNote = ""

	got := unblockJSONResult(t, festDir, taskRel)
	want := map[string]any{
		"success": true,
		"task":    canonicalTaskID(t, festDir, taskRel),
		"cleared": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unblock --json = %v, want the pre-change shape %v", got, want)
	}
}

func TestUnblockJSONCarriesTheNoteWhenSet(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)
	unblockNote = "the v2 endpoint does the same thing, try that"

	got := unblockJSONResult(t, festDir, taskRel)
	if got["note"] != "the v2 endpoint does the same thing, try that" {
		t.Errorf("unblock --json note = %v, want the operator note", got["note"])
	}
}

func TestUnblockCmdIsNotGuarded(t *testing.T) {
	cmd := newUnblockCmd()
	if cmd.Flags().Lookup("json") == nil {
		t.Error("unblock must keep --json; it is the frictionless reject path")
	}
	if cmd.Flags().Lookup("note") == nil {
		t.Error("unblock must offer --note")
	}
	if !strings.Contains(cmd.Long, "Pass --note to tell the executor what to try") {
		t.Errorf("help text does not explain --note:\n%s", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "does not prompt for") {
		t.Errorf("help text must keep the frictionless promise:\n%s", cmd.Long)
	}
}

// TestUnblockWorksUnderEveryGuardRefusalCondition proves the reject path stays
// available exactly where the defer path is refused: no terminal, every agent
// marker set, and an agent binary in the parent chain.
func TestUnblockWorksUnderEveryGuardRefusalCondition(t *testing.T) {
	festDir, taskRel := deferHookedFixture(t)

	stubOperatorGuardTerminal(t, false)
	stubOperatorGuardAncestry(t, []string{"claude"}, nil)
	for _, marker := range operatorAgentMarkers {
		t.Setenv(marker, "1")
	}
	unblockNote = "the v2 endpoint does the same thing, try that"
	t.Cleanup(func() { unblockNote = "" })

	captureIO(t, func() {
		if err := runUnblock(taskCmd(t, festDir), []string{taskRel}); err != nil {
			t.Fatalf("runUnblock() error = %v; unblock must never be guarded", err)
		}
	})

	events := readHookEvents(t, festDir)
	if !strings.Contains(events, `"note":"the v2 endpoint does the same thing, try that"`) {
		t.Errorf("the note is missing from the event log:\n%s", events)
	}
}
