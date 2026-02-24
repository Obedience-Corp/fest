package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

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
				Version:  1,
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

// WalkCommands recursively traverses the command tree and collects
// commands that have agent restriction annotations.
func WalkCommands(cmd *cobra.Command, prefix string, entries *[]CommandEntry) {
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		path := child.Name()
		if prefix != "" {
			path = prefix + " " + child.Name()
		}

		if val, ok := child.Annotations["agent_allowed"]; ok {
			entry := CommandEntry{
				Path:         path,
				AgentAllowed: val == "true",
			}
			if reason, ok := child.Annotations["agent_reason"]; ok {
				entry.Reason = reason
			}
			if interactive, ok := child.Annotations["interactive"]; ok {
				entry.Interactive = interactive == "true"
			}
			*entries = append(*entries, entry)
		}

		WalkCommands(child, path, entries)
	}
}
