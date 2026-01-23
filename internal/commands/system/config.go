package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	uitheme "github.com/Obedience-Corp/fest/internal/ui/theme"
)

// NewConfigCommand creates the config management command
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage fest configuration settings",
		Long: `Interactive TUI for managing fest configuration.

All settings are displayed in a flat list for easy editing.
Changes are saved to ~/.config/fest/config.json.

Use arrow keys or j/k to navigate, Enter to edit, Esc to exit.`,
		Example: `  fest system config           # Open configuration TUI
  fest system config --show    # Display current configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			show, _ := cmd.Flags().GetBool("show")
			if show {
				return showCurrentConfig(cmd.Context())
			}
			return runConfigTUI(cmd.Context())
		},
	}

	cmd.Flags().Bool("show", false, "display current configuration as JSON")

	return cmd
}

// runConfigTUI runs the flat configuration settings form
func runConfigTUI(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cfg, err := config.Load(ctx, "")
	if err != nil {
		return errors.Wrap(err, "loading config")
	}

	for {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}

		// Convert int to string for form input
		maxHeightStr := strconv.Itoa(cfg.TUI.MaxInputHeight)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Repository URL").
					Description("GitHub repository for festival methodology templates").
					Placeholder(config.DefaultRepositoryURL).
					Value(&cfg.Repository.URL),
				huh.NewInput().
					Title("Repository Branch").
					Description("Git branch to sync from").
					Placeholder("main").
					Value(&cfg.Repository.Branch),
				huh.NewInput().
					Title("Repository Path").
					Description("Path within repository to methodology files").
					Placeholder(config.DefaultRepoPath).
					Value(&cfg.Repository.Path),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Editor").
					Description("For wizard fill (empty = $EDITOR or vim)").
					Placeholder("vim").
					Value(&cfg.Behavior.Editor),
			),
			huh.NewGroup(
				huh.NewConfirm().
					Title("Vim Mode").
					Description("Enable vim-style keybindings (j/k navigation)").
					Value(&cfg.TUI.VimMode),
				huh.NewConfirm().
					Title("Expand Inputs").
					Description("Auto-expand text areas as content grows").
					Value(&cfg.TUI.ExpandInputs),
				huh.NewInput().
					Title("Max Input Height").
					Description("Maximum lines for expandable text areas").
					Placeholder("10").
					Value(&maxHeightStr).
					Validate(validatePositiveInt),
				huh.NewSelect[string]().
					Title("Theme").
					Description("Color theme for TUI elements").
					Options(
						huh.NewOption("adaptive", "adaptive"),
						huh.NewOption("light", "light"),
						huh.NewOption("dark", "dark"),
						huh.NewOption("high-contrast", "high-contrast"),
					).
					Value(&cfg.TUI.Theme),
			),
		)

		if err := uitheme.RunForm(ctx, form); err != nil {
			if uitheme.IsCancelled(err) {
				return nil // ESC exits cleanly
			}
			return err
		}

		// Parse max height back to int
		cfg.TUI.MaxInputHeight, _ = strconv.Atoi(maxHeightStr)

		// Save the configuration
		if err := config.Save(ctx, cfg); err != nil {
			return errors.Wrap(err, "saving config")
		}

		// Show confirmation and loop back
		display := ui.New(false, false)
		display.Success("Settings saved")
		fmt.Println()
	}
}

// showCurrentConfig displays the current configuration as JSON
func showCurrentConfig(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cfg, err := config.Load(ctx, "")
	if err != nil {
		return errors.Wrap(err, "loading config")
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling config")
	}

	display := ui.New(false, false)
	display.Info("Configuration file: %s/config.json", config.ConfigDir())
	fmt.Println()
	fmt.Println(string(data))
	fmt.Println()

	return nil
}

// validatePositiveInt validates that input is a positive integer
func validatePositiveInt(s string) error {
	if s == "" {
		return nil // Allow empty (will use default)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n < 0 {
		return fmt.Errorf("must be positive")
	}
	return nil
}
