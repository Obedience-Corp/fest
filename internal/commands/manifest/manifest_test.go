package manifest

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

// newMockCommandTree creates a command tree with both annotated and unannotated commands
// for testing WalkCommands behavior.
func newMockCommandTree() *cobra.Command {
	root := &cobra.Command{Use: "fest"}

	// A restricted command
	restricted := &cobra.Command{
		Use: "restricted-cmd",
		Annotations: map[string]string{
			"agent_allowed": "false",
			"agent_reason":  "Test restriction reason",
			"interactive":   "true",
		},
	}

	// An allowed command (no annotations)
	allowed := &cobra.Command{
		Use: "allowed-cmd",
	}

	// A parent with a restricted child
	parent := &cobra.Command{
		Use: "parent",
	}
	restrictedChild := &cobra.Command{
		Use: "restricted-child",
		Annotations: map[string]string{
			"agent_allowed": "false",
			"agent_reason":  "Child restriction reason",
		},
	}
	allowedChild := &cobra.Command{
		Use: "allowed-child",
	}
	parent.AddCommand(restrictedChild)
	parent.AddCommand(allowedChild)

	root.AddCommand(restricted)
	root.AddCommand(allowed)
	root.AddCommand(parent)

	return root
}

func TestWalkCommands_CollectsAnnotatedCommands(t *testing.T) {
	root := newMockCommandTree()

	var entries []CommandEntry
	WalkCommands(root, "", &entries)

	if len(entries) == 0 {
		t.Fatal("WalkCommands returned no entries from annotated command tree")
	}

	// Should collect exactly 2: restricted-cmd and parent restricted-child
	if len(entries) != 2 {
		t.Errorf("expected 2 annotated commands, got %d", len(entries))
	}
}

func TestWalkCommands_IgnoresUnannotatedCommands(t *testing.T) {
	root := newMockCommandTree()

	var entries []CommandEntry
	WalkCommands(root, "", &entries)

	for _, entry := range entries {
		if entry.Path == "allowed-cmd" {
			t.Error("unannotated command 'allowed-cmd' should not appear in manifest")
		}
		if entry.Path == "parent allowed-child" {
			t.Error("unannotated command 'parent allowed-child' should not appear in manifest")
		}
	}
}

func TestWalkCommands_BuildsCorrectSubcommandPaths(t *testing.T) {
	root := newMockCommandTree()

	var entries []CommandEntry
	WalkCommands(root, "", &entries)

	pathMap := make(map[string]bool)
	for _, entry := range entries {
		pathMap[entry.Path] = true
	}

	if !pathMap["restricted-cmd"] {
		t.Error("expected top-level path 'restricted-cmd' not found")
	}
	if !pathMap["parent restricted-child"] {
		t.Error("expected subcommand path 'parent restricted-child' not found")
	}
}

func TestWalkCommands_ExtractsAnnotationFields(t *testing.T) {
	root := newMockCommandTree()

	var entries []CommandEntry
	WalkCommands(root, "", &entries)

	// Find the restricted-cmd entry
	var found bool
	for _, entry := range entries {
		if entry.Path == "restricted-cmd" {
			found = true
			if entry.AgentAllowed {
				t.Error("expected agent_allowed=false")
			}
			if entry.Reason != "Test restriction reason" {
				t.Errorf("expected reason 'Test restriction reason', got %q", entry.Reason)
			}
			if !entry.Interactive {
				t.Error("expected interactive=true")
			}
			break
		}
	}
	if !found {
		t.Fatal("restricted-cmd not found in entries")
	}

	// Find the child entry - should NOT have interactive set
	for _, entry := range entries {
		if entry.Path == "parent restricted-child" {
			if entry.Interactive {
				t.Error("child command should not have interactive=true")
			}
			if entry.Reason != "Child restriction reason" {
				t.Errorf("expected reason 'Child restriction reason', got %q", entry.Reason)
			}
		}
	}
}

func TestManifest_JSONMarshal(t *testing.T) {
	m := Manifest{
		Version: 1,
		CLI:     "fest",
		Commands: []CommandEntry{
			{
				Path:         "tui",
				AgentAllowed: false,
				Reason:       "Full interactive Charm TUI",
				Interactive:  true,
			},
		},
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	var parsed Manifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse marshaled manifest: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.CLI != "fest" {
		t.Errorf("expected cli 'fest', got %q", parsed.CLI)
	}
	if len(parsed.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(parsed.Commands))
	}
	if parsed.Commands[0].Path != "tui" {
		t.Errorf("expected path 'tui', got %q", parsed.Commands[0].Path)
	}
}

func TestExpectedRestrictedCommands(t *testing.T) {
	expected := []string{
		"tui",
		"create",
		"config",
		"shell-init",
		"wizard fill",
		"system config",
		"system sync",
		"system update",
	}

	if len(expected) != 8 {
		t.Errorf("expected 8 restricted commands, got %d", len(expected))
	}
}
