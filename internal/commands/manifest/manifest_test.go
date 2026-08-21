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

	restricted := &cobra.Command{
		Use: "restricted-cmd",
		Annotations: map[string]string{
			"agent_allowed": "false",
			"agent_reason":  "Test restriction reason",
			"interactive":   "true",
		},
	}

	allowed := &cobra.Command{
		Use: "allowed-cmd",
	}

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

	hidden := &cobra.Command{
		Use:    "hidden-cmd",
		Hidden: true,
		Annotations: map[string]string{
			"agent_allowed": "true",
		},
	}
	help := &cobra.Command{Use: "help"}
	completion := &cobra.Command{Use: "completion"}
	completion.AddCommand(&cobra.Command{Use: "bash"})

	root.AddCommand(restricted)
	root.AddCommand(allowed)
	root.AddCommand(parent)
	root.AddCommand(hidden)
	root.AddCommand(help)
	root.AddCommand(completion)

	return root
}

func walkMap(t *testing.T, root *cobra.Command) map[string]CommandEntry {
	t.Helper()
	var entries []CommandEntry
	WalkCommands(root, "", &entries)
	out := make(map[string]CommandEntry, len(entries))
	for _, entry := range entries {
		out[entry.Path] = entry
	}
	return out
}

func TestWalkCommands_EmitsExhaustiveAllowlist(t *testing.T) {
	got := walkMap(t, newMockCommandTree())

	wantPaths := []string{
		"restricted-cmd",
		"allowed-cmd",
		"parent",
		"parent restricted-child",
		"parent allowed-child",
	}
	if len(got) != len(wantPaths) {
		t.Fatalf("expected %d commands, got %d: %v", len(wantPaths), len(got), keys(got))
	}
	for _, path := range wantPaths {
		if _, ok := got[path]; !ok {
			t.Errorf("missing command %q", path)
		}
	}
}

func TestWalkCommands_UnannotatedDefaultAllowed(t *testing.T) {
	got := walkMap(t, newMockCommandTree())

	for _, path := range []string{"allowed-cmd", "parent", "parent allowed-child"} {
		entry, ok := got[path]
		if !ok {
			t.Fatalf("unannotated command %q missing from v2 manifest", path)
		}
		if !entry.AgentAllowed {
			t.Errorf("unannotated command %q should default to agent_allowed=true", path)
		}
	}
}

func TestWalkCommands_OmitsHiddenHelpAndCompletion(t *testing.T) {
	got := walkMap(t, newMockCommandTree())
	for _, path := range []string{"hidden-cmd", "help", "completion", "completion bash"} {
		if _, ok := got[path]; ok {
			t.Errorf("command %q should be omitted from the agent manifest", path)
		}
	}
}

func TestWalkCommands_BuildsCorrectSubcommandPaths(t *testing.T) {
	got := walkMap(t, newMockCommandTree())
	if _, ok := got["restricted-cmd"]; !ok {
		t.Error("expected top-level path 'restricted-cmd' not found")
	}
	if _, ok := got["parent restricted-child"]; !ok {
		t.Error("expected subcommand path 'parent restricted-child' not found")
	}
}

func TestWalkCommands_ExtractsAnnotationFields(t *testing.T) {
	got := walkMap(t, newMockCommandTree())

	restricted, ok := got["restricted-cmd"]
	if !ok {
		t.Fatal("restricted-cmd not found in entries")
	}
	if restricted.AgentAllowed {
		t.Error("expected agent_allowed=false")
	}
	if restricted.Reason != "Test restriction reason" {
		t.Errorf("expected reason 'Test restriction reason', got %q", restricted.Reason)
	}
	if !restricted.Interactive {
		t.Error("expected interactive=true")
	}

	child, ok := got["parent restricted-child"]
	if !ok {
		t.Fatal("parent restricted-child not found")
	}
	if child.Interactive {
		t.Error("child command should not have interactive=true")
	}
	if child.Reason != "Child restriction reason" {
		t.Errorf("expected reason 'Child restriction reason', got %q", child.Reason)
	}
}

func TestManifest_JSONMarshal(t *testing.T) {
	m := Manifest{
		Version: AgentGatingVersion,
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

	if parsed.Version != AgentGatingVersion {
		t.Errorf("expected version %d, got %d", AgentGatingVersion, parsed.Version)
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

func TestRestrictedPaths(t *testing.T) {
	if len(RestrictedPaths) != 12 {
		t.Errorf("expected 12 restricted commands, got %d", len(RestrictedPaths))
	}
}

func keys(m map[string]CommandEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
