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

Navigate to a setting to edit it. Changes are saved immediately.
Configuration is stored in ~/.config/fest/config.json.

Use arrow keys or j/k to navigate, Enter to select, Esc to exit.`,
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

// runConfigTUI runs the navigable settings menu
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

		// Build menu options showing current values
		options := []huh.Option[string]{
			huh.NewOption(fmt.Sprintf("Repository URL      %s", truncateStr(cfg.Repository.URL, 30)), "repo_url"),
			huh.NewOption(fmt.Sprintf("Repository Branch   %s", displayStr(cfg.Repository.Branch, "main")), "repo_branch"),
			huh.NewOption(fmt.Sprintf("Repository Path     %s", displayStr(cfg.Repository.Path, ".festival")), "repo_path"),
			huh.NewOption("─────────────────────", ""),
			huh.NewOption(fmt.Sprintf("Editor              %s", displayStr(cfg.Behavior.Editor, "$EDITOR")), "editor"),
			huh.NewOption("─────────────────────", ""),
			huh.NewOption(fmt.Sprintf("Vim Mode            %s", boolDisplay(cfg.TUI.VimMode)), "vim_mode"),
			huh.NewOption(fmt.Sprintf("Expand Inputs       %s", boolDisplay(cfg.TUI.ExpandInputs)), "expand_inputs"),
			huh.NewOption(fmt.Sprintf("Max Input Height    %d", cfg.TUI.MaxInputHeight), "max_height"),
			huh.NewOption(fmt.Sprintf("Theme               %s", cfg.TUI.Theme), "theme"),
			huh.NewOption("─────────────────────", ""),
			huh.NewOption("Exit", "exit"),
		}

		var choice string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Configuration Settings").
				Description("Select a setting to edit").
				Options(options...).
				Value(&choice),
		))

		if err := uitheme.RunForm(ctx, form); err != nil {
			if uitheme.IsCancelled(err) {
				return nil
			}
			return err
		}

		switch choice {
		case "exit", "":
			return nil
		case "repo_url":
			editStringSetting(ctx, cfg, "Repository URL", "GitHub repository for festival methodology templates", &cfg.Repository.URL)
		case "repo_branch":
			editStringSetting(ctx, cfg, "Repository Branch", "Git branch to sync from", &cfg.Repository.Branch)
		case "repo_path":
			editStringSetting(ctx, cfg, "Repository Path", "Path within repository to methodology files", &cfg.Repository.Path)
		case "editor":
			editStringSetting(ctx, cfg, "Editor", "Preferred editor for wizard fill (empty = $EDITOR or vim)", &cfg.Behavior.Editor)
		case "vim_mode":
			editBoolSetting(ctx, cfg, "Vim Mode", "Enable vim-style keybindings (j/k navigation)", &cfg.TUI.VimMode)
		case "expand_inputs":
			editBoolSetting(ctx, cfg, "Expand Inputs", "Auto-expand text areas as content grows", &cfg.TUI.ExpandInputs)
		case "max_height":
			editIntSetting(ctx, cfg, "Max Input Height", "Maximum lines for expandable text areas", &cfg.TUI.MaxInputHeight)
		case "theme":
			editThemeSetting(ctx, cfg)
		}
	}
}

// editStringSetting prompts to edit a single string setting
func editStringSetting(ctx context.Context, cfg *config.Config, title, description string, value *string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(title).
			Description(description).
			Value(value),
	))

	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return nil // Don't save on cancel
		}
		return err
	}

	return config.Save(ctx, cfg)
}

// editBoolSetting prompts to edit a single boolean setting
func editBoolSetting(ctx context.Context, cfg *config.Config, title, description string, value *bool) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(description).
			Value(value),
	))

	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return nil
		}
		return err
	}

	return config.Save(ctx, cfg)
}

// editIntSetting prompts to edit a single integer setting
func editIntSetting(ctx context.Context, cfg *config.Config, title, description string, value *int) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	strValue := strconv.Itoa(*value)
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(title).
			Description(description).
			Value(&strValue).
			Validate(validatePositiveInt),
	))

	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return nil
		}
		return err
	}

	*value, _ = strconv.Atoi(strValue)
	return config.Save(ctx, cfg)
}

// editThemeSetting prompts to select a theme
func editThemeSetting(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Theme").
			Description("Color theme for TUI elements").
			Options(
				huh.NewOption("adaptive - Auto-detect light/dark", "adaptive"),
				huh.NewOption("light - Optimized for light backgrounds", "light"),
				huh.NewOption("dark - Optimized for dark backgrounds", "dark"),
				huh.NewOption("high-contrast - Maximum visibility", "high-contrast"),
			).
			Value(&cfg.TUI.Theme),
	))

	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return nil
		}
		return err
	}

	return config.Save(ctx, cfg)
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

// Helper functions

// truncateStr truncates a string to max length with ellipsis
func truncateStr(s string, max int) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// displayStr returns the string or a default if empty
func displayStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// boolDisplay returns "enabled" or "disabled"
func boolDisplay(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

// validatePositiveInt validates that input is a positive integer
func validatePositiveInt(s string) error {
	if s == "" {
		return nil
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
