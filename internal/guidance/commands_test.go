package guidance

import (
	"testing"
)

func TestDefaultModeCommands_AllModesPresent(t *testing.T) {
	// Verify all modes have entries in the registry
	modes := AllModes()
	for _, mode := range modes {
		cmds, ok := DefaultModeCommands[mode]
		if !ok {
			t.Errorf("DefaultModeCommands missing entry for mode %q", mode)
			continue
		}
		if cmds.DisplayName == "" {
			t.Errorf("Mode %q has empty DisplayName", mode)
		}
		if cmds.StartCommand == "" {
			t.Errorf("Mode %q has empty StartCommand", mode)
		}
		if cmds.NextCommand == "" {
			t.Errorf("Mode %q has empty NextCommand", mode)
		}
		if cmds.CompleteCommand == "" {
			t.Errorf("Mode %q has empty CompleteCommand", mode)
		}
	}
}

func TestGetModeCommands(t *testing.T) {
	tests := []struct {
		name            string
		mode            Mode
		wantDisplayName string
	}{
		{
			name:            "implementation mode",
			mode:            ModeImplementation,
			wantDisplayName: "Implementation Mode",
		},
		{
			name:            "plan mode",
			mode:            ModePlan,
			wantDisplayName: "Planning Mode",
		},
		{
			name:            "research mode",
			mode:            ModeResearch,
			wantDisplayName: "Research Mode",
		},
		{
			name:            "review mode",
			mode:            ModeReview,
			wantDisplayName: "Review Mode",
		},
		{
			name:            "action mode",
			mode:            ModeAction,
			wantDisplayName: "Action Mode",
		},
		{
			name:            "ingest mode",
			mode:            ModeIngest,
			wantDisplayName: "Ingest Mode",
		},
		{
			name:            "unknown mode defaults to implementation",
			mode:            Mode("unknown"),
			wantDisplayName: "Implementation Mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := GetModeCommands(tt.mode)
			if cmds.DisplayName != tt.wantDisplayName {
				t.Errorf("GetModeCommands(%q).DisplayName = %q, want %q",
					tt.mode, cmds.DisplayName, tt.wantDisplayName)
			}
		})
	}
}

func TestFormatProgressCommand(t *testing.T) {
	tests := []struct {
		name     string
		taskPath string
		want     string
	}{
		{
			name:     "simple path",
			taskPath: "task.md",
			want:     "fest task completed task.md",
		},
		{
			name:     "nested path",
			taskPath: "001_PHASE/01_seq/01_task.md",
			want:     "fest task completed 001_PHASE/01_seq/01_task.md",
		},
		{
			name:     "absolute path",
			taskPath: "/path/to/task.md",
			want:     "fest task completed /path/to/task.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatProgressCommand(tt.taskPath)
			if got != tt.want {
				t.Errorf("FormatProgressCommand(%q) = %q, want %q",
					tt.taskPath, got, tt.want)
			}
		})
	}
}

func TestModeCommands_AdditionalCmds(t *testing.T) {
	// Verify modes with AdditionalCmds have expected keys
	reviewCmds := GetModeCommands(ModeReview)
	if reviewCmds.AdditionalCmds == nil {
		t.Fatal("Review mode should have AdditionalCmds")
	}
	for _, key := range []string{"pass", "fail", "skip"} {
		if _, ok := reviewCmds.AdditionalCmds[key]; !ok {
			t.Errorf("Review mode missing AdditionalCmds key %q", key)
		}
	}

	actionCmds := GetModeCommands(ModeAction)
	if actionCmds.AdditionalCmds == nil {
		t.Fatal("Action mode should have AdditionalCmds")
	}
	for _, key := range []string{"complete", "fail", "skip"} {
		if _, ok := actionCmds.AdditionalCmds[key]; !ok {
			t.Errorf("Action mode missing AdditionalCmds key %q", key)
		}
	}

	ingestCmds := GetModeCommands(ModeIngest)
	if ingestCmds.AdditionalCmds == nil {
		t.Fatal("Ingest mode should have AdditionalCmds")
	}
	for _, key := range []string{"approve", "reject"} {
		if _, ok := ingestCmds.AdditionalCmds[key]; !ok {
			t.Errorf("Ingest mode missing AdditionalCmds key %q", key)
		}
	}
}
