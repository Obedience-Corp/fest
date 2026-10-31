package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/hooks"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

type notificationView struct {
	Name    string     `json:"name"`
	Event   hooks.Verb `json:"event"`
	Enabled bool       `json:"enabled"`
	Command string     `json:"command,omitempty"`
	Timeout string     `json:"timeout,omitempty"`
	Error   string     `json:"error,omitempty"`
}

func newNotificationsCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use: "notifications", Short: "Inspect machine-local workflow notifications",
		Long: `Inspect presentation_open (entry into PRESENT) and approval_notify (successful approval).
These optional notifications run after command execution and lock release. They never
approve checkpoints. Only explicitly enabled machine definitions apply; workspace and
festival definitions are ignored. Notifications always fail open and have bounded timeouts.
They are separate from the ordinary hooks shown by 'fest hooks list'.`,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"scope": string(scope.Global)},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cmd.Context(), "")
			if err != nil {
				return err
			}
			var definitions *config.HooksConfig
			if cfg != nil {
				definitions = cfg.Hooks
			}
			views := []notificationView{}
			for _, spec := range []struct {
				name string
				verb hooks.Verb
			}{
				{hooks.PresentationOpenName, hooks.VerbWorkflowPresent},
				{hooks.ApprovalNotifyName, hooks.VerbWorkflowApproved},
			} {
				hook, enabled, resolveErr := hooks.ResolveNotification(definitions, spec.name)
				view := notificationView{Name: spec.name, Event: spec.verb, Enabled: enabled}
				if definitions != nil {
					view.Command = definitions.Definitions[spec.name].Command
				}
				if enabled {
					view.Timeout = hook.Timeout.String()
				}
				if resolveErr != nil {
					view.Error = resolveErr.Error()
				}
				views = append(views, view)
			}
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(views)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Machine-local notifications (shared definitions are ignored):")
			for _, view := range views {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: enabled=%t event=%s\n", view.Name, view.Enabled, view.Event)
				if view.Command != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  command: %s\n", view.Command)
				}
				if view.Error != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  invalid: %s\n", view.Error)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	return cmd
}
