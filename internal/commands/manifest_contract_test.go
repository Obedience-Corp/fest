package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	manifestcmd "github.com/Obedience-Corp/fest/internal/commands/manifest"
)

func TestManifestCommand_EmitsVersion2ExhaustiveAllowlist(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"__manifest"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("__manifest: %v\n%s", err, buf.String())
	}

	var m manifestcmd.Manifest
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("parse __manifest JSON: %v\n%s", err, buf.String())
	}
	if m.Version != manifestcmd.AgentGatingVersion {
		t.Errorf("version = %d, want %d", m.Version, manifestcmd.AgentGatingVersion)
	}
	if m.CLI != "fest" {
		t.Errorf("cli = %q, want fest", m.CLI)
	}
	if len(m.Commands) == 0 {
		t.Fatal("manifest listed no commands")
	}

	byPath := make(map[string]manifestcmd.CommandEntry, len(m.Commands))
	for _, entry := range m.Commands {
		byPath[entry.Path] = entry
	}

	for _, path := range manifestcmd.RestrictedPaths {
		entry, ok := byPath[path]
		if !ok {
			// tui is registered only on the dev channel.
			if path == "tui" {
				continue
			}
			t.Errorf("restricted path %q missing from v2 manifest", path)
			continue
		}
		if entry.AgentAllowed {
			t.Errorf("restricted path %q is agent_allowed=true", path)
		}
		if entry.Reason == "" {
			t.Errorf("restricted path %q has empty reason", path)
		}
	}

	for _, path := range []string{"status", "validate", "next", "task completed"} {
		entry, ok := byPath[path]
		if !ok {
			t.Errorf("expected agent-facing path %q in manifest", path)
			continue
		}
		if !entry.AgentAllowed {
			t.Errorf("path %q should default to agent_allowed=true", path)
		}
	}

	var walked []manifestcmd.CommandEntry
	manifestcmd.WalkCommands(rootCmd, "", &walked)
	if len(walked) != len(m.Commands) {
		t.Errorf("JSON listed %d commands, WalkCommands produced %d", len(m.Commands), len(walked))
	}

	for _, path := range []string{"__manifest", "__commands", "help", "completion", "gendocs"} {
		if _, ok := byPath[path]; ok {
			t.Errorf("omitted path %q leaked into the agent manifest", path)
		}
	}
}
