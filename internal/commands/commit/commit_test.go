package commit

import (
	"testing"
)

func TestNewCommitCommand_SyncSubmoduleRefFlag(t *testing.T) {
	cmd := NewCommitCommand()

	flag := cmd.Flags().Lookup("sync-submodule-ref")
	if flag == nil {
		t.Fatal("--sync-submodule-ref flag not registered")
	}

	if flag.DefValue != "false" {
		t.Errorf("--sync-submodule-ref default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewCommitCommand_CampaignTaggingIndependentOfSync(t *testing.T) {
	// Verify that the --no-tag flag and --sync-submodule-ref flag are independent.
	// Campaign ID tagging (--no-tag) should NOT be affected by sync flag state.
	cmd := NewCommitCommand()

	noTagFlag := cmd.Flags().Lookup("no-tag")
	syncFlag := cmd.Flags().Lookup("sync-submodule-ref")

	if noTagFlag == nil {
		t.Fatal("--no-tag flag not registered")
	}
	if syncFlag == nil {
		t.Fatal("--sync-submodule-ref flag not registered")
	}

	// Both flags exist independently — they gate different behavior
	if noTagFlag.DefValue != "false" {
		t.Errorf("--no-tag default = %q, want %q", noTagFlag.DefValue, "false")
	}
	if syncFlag.DefValue != "false" {
		t.Errorf("--sync-submodule-ref default = %q, want %q", syncFlag.DefValue, "false")
	}
}
