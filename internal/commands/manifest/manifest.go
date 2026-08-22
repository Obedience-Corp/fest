package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// AgentGatingVersion is the exhaustive allowlist contract: every visible
// command path carries an agent_allowed verdict and absence fails closed
// for consumers that enforce version 2 (obey RunAgentAllowed). Version 1
// enumerated only refusals and must not be read with allowlist semantics.
const AgentGatingVersion = 2

// RestrictedPaths remain agent_allowed=false under the v2 exhaustive
// allowlist. They are the historical v1 denylist: interactive TUIs,
// operator-only overrides, and shell/config surfaces that agents must
// not drive.
var RestrictedPaths = []string{
	"tui",
	"create",
	"config",
	"config add",
	"config sync",
	"config use",
	"shell-init",
	"wizard fill",
	"workflow skip",
	"system config",
	"system sync",
	"system update",
}

// Manifest represents the CLI command restriction manifest.
type Manifest struct {
	Version  int            `json:"version"`
	CLI      string         `json:"cli"`
	Commands []CommandEntry `json:"commands"`
}

// CommandEntry represents a single command's agent restriction metadata.
type CommandEntry struct {
	Path         string `json:"path"`
	AgentAllowed bool   `json:"agent_allowed"`
	Reason       string `json:"reason,omitempty"`
	Interactive  bool   `json:"interactive,omitempty"`
}

// NewManifestCommand creates the hidden __manifest command.
func NewManifestCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "__manifest",
		Short:  "Output command manifest for daemon enforcement",
		Hidden: true,
		Annotations: map[string]string{
			"scope": "global",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			m := Manifest{
				Version:  AgentGatingVersion,
				CLI:      "fest",
				Commands: []CommandEntry{},
			}
			WalkCommands(cmd.Root(), "", &m.Commands)
			data, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		},
	}
}

// WalkCommands recursively traverses the command tree and emits a verdict
// for every visible command. Annotated commands keep their agent_allowed
// value; unannotated commands default to allowed so the v2 document
// preserves the v1 implicit-allow policy rather than locking agents out
// of the rest of the CLI. Hidden commands, cobra help, and completion
// are omitted (they are not an agent surface).
func WalkCommands(cmd *cobra.Command, prefix string, entries *[]CommandEntry) {
	for _, child := range cmd.Commands() {
		if skipManifestCommand(child) {
			continue
		}
		path := child.Name()
		if prefix != "" {
			path = prefix + " " + child.Name()
		}

		entry := CommandEntry{
			Path:         path,
			AgentAllowed: true,
		}
		if val, ok := child.Annotations["agent_allowed"]; ok {
			entry.AgentAllowed = val == "true"
		}
		if reason, ok := child.Annotations["agent_reason"]; ok {
			entry.Reason = reason
		}
		if interactive, ok := child.Annotations["interactive"]; ok {
			entry.Interactive = interactive == "true"
		}
		*entries = append(*entries, entry)

		WalkCommands(child, path, entries)
	}
}

func skipManifestCommand(cmd *cobra.Command) bool {
	if cmd.Hidden {
		return true
	}
	switch cmd.Name() {
	case "help", "completion":
		return true
	default:
		return false
	}
}
