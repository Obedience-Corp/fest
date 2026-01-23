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

Navigate through configuration categories and modify settings.
Changes are saved to ~/.config/fest/config.json.

Use arrow keys or j/k to navigate, Enter to select, Esc to go back.`,
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

// runConfigTUI runs the main configuration TUI loop
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

		var action string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Configuration Settings").
					Description("Select a category to configure").
					Options(
						huh.NewOption("Behavior Settings", "behavior"),
						huh.NewOption("Network Settings", "network"),
						huh.NewOption("TUI Settings", "tui"),
						huh.NewOption("Repository Settings", "repository"),
						huh.NewOption("Local Paths", "local"),
						huh.NewOption("─────────────────", ""),
						huh.NewOption("View Current Config", "view"),
						huh.NewOption("Reset to Defaults", "reset"),
						huh.NewOption("Exit", "exit"),
					).
					Value(&action),
			),
		)

		if err := uitheme.RunForm(ctx, form); err != nil {
			if uitheme.IsCancelled(err) {
				return nil
			}
			return err
		}

		switch action {
		case "behavior":
			if err := editBehaviorSettings(ctx, cfg); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
		case "network":
			if err := editNetworkSettings(ctx, cfg); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
		case "tui":
			if err := editTUISettings(ctx, cfg); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
		case "repository":
			if err := editRepositorySettings(ctx, cfg); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
		case "local":
			if err := editLocalPathSettings(ctx, cfg); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
		case "view":
			if err := showCurrentConfig(ctx); err != nil {
				return err
			}
		case "reset":
			if err := resetToDefaults(ctx); err != nil {
				if uitheme.IsCancelled(err) {
					continue
				}
				return err
			}
			// Reload config after reset
			cfg, err = config.Load(ctx, "")
			if err != nil {
				return errors.Wrap(err, "reloading config after reset")
			}
		case "exit", "":
			return nil
		}
	}
}

// editBehaviorSettings edits behavior configuration
func editBehaviorSettings(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Auto Backup").
				Description("Automatically backup files before updates").
				Value(&cfg.Behavior.AutoBackup),
			huh.NewConfirm().
				Title("Interactive Mode").
				Description("Prompt for confirmations during operations").
				Value(&cfg.Behavior.Interactive),
			huh.NewConfirm().
				Title("Use Color").
				Description("Enable colored output (respects NO_COLOR env)").
				Value(&cfg.Behavior.UseColor),
			huh.NewConfirm().
				Title("Verbose Output").
				Description("Show detailed operation information").
				Value(&cfg.Behavior.Verbose),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Editor").
				Description("Preferred editor for wizard fill (empty = $EDITOR or vim)").
				Placeholder("vim").
				Value(&cfg.Behavior.Editor),
			huh.NewSelect[string]().
				Title("Editor Mode").
				Description("How to open editor for wizard fill").
				Options(
					huh.NewOption("buffer - Clean single window", "buffer"),
					huh.NewOption("tab - Open in new tab", "tab"),
					huh.NewOption("split - Vertical split", "split"),
					huh.NewOption("hsplit - Horizontal split", "hsplit"),
				).
				Value(&cfg.Behavior.EditorMode),
		),
	)

	if err := uitheme.RunForm(ctx, form); err != nil {
		return err
	}

	return config.Save(ctx, cfg)
}

// editNetworkSettings edits network configuration
func editNetworkSettings(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	timeoutStr := strconv.Itoa(cfg.Network.Timeout)
	retryCountStr := strconv.Itoa(cfg.Network.RetryCount)
	retryDelayStr := strconv.Itoa(cfg.Network.RetryDelay)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Timeout (seconds)").
				Description("HTTP request timeout for sync operations").
				Placeholder("30").
				Value(&timeoutStr).
				Validate(validatePositiveInt),
			huh.NewInput().
				Title("Retry Count").
				Description("Number of retry attempts for failed requests").
				Placeholder("3").
				Value(&retryCountStr).
				Validate(validatePositiveInt),
			huh.NewInput().
				Title("Retry Delay (seconds)").
				Description("Delay between retry attempts").
				Placeholder("1").
				Value(&retryDelayStr).
				Validate(validatePositiveInt),
		),
	)

	if err := uitheme.RunForm(ctx, form); err != nil {
		return err
	}

	// Parse values back to integers
	cfg.Network.Timeout, _ = strconv.Atoi(timeoutStr)
	cfg.Network.RetryCount, _ = strconv.Atoi(retryCountStr)
	cfg.Network.RetryDelay, _ = strconv.Atoi(retryDelayStr)

	return config.Save(ctx, cfg)
}

// editTUISettings edits TUI configuration
func editTUISettings(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	maxHeightStr := strconv.Itoa(cfg.TUI.MaxInputHeight)

	form := huh.NewForm(
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
					huh.NewOption("adaptive - Auto-detect light/dark", "adaptive"),
					huh.NewOption("light - Optimized for light backgrounds", "light"),
					huh.NewOption("dark - Optimized for dark backgrounds", "dark"),
					huh.NewOption("high-contrast - Maximum visibility", "high-contrast"),
				).
				Value(&cfg.TUI.Theme),
		),
	)

	if err := uitheme.RunForm(ctx, form); err != nil {
		return err
	}

	cfg.TUI.MaxInputHeight, _ = strconv.Atoi(maxHeightStr)

	return config.Save(ctx, cfg)
}

// editRepositorySettings edits repository configuration
func editRepositorySettings(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Repository URL").
				Description("GitHub repository for festival methodology templates").
				Placeholder(config.DefaultRepositoryURL).
				Value(&cfg.Repository.URL),
			huh.NewInput().
				Title("Branch").
				Description("Git branch to sync from").
				Placeholder("main").
				Value(&cfg.Repository.Branch),
			huh.NewInput().
				Title("Repository Path").
				Description("Path within repository to methodology files").
				Placeholder(config.DefaultRepoPath).
				Value(&cfg.Repository.Path),
		),
	)

	if err := uitheme.RunForm(ctx, form); err != nil {
		return err
	}

	return config.Save(ctx, cfg)
}

// editLocalPathSettings edits local path configuration
func editLocalPathSettings(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Cache Directory").
				Description("Directory for cached template downloads").
				Placeholder("~/.config/fest/cache").
				Value(&cfg.Local.CacheDir),
			huh.NewInput().
				Title("Backup Directory").
				Description("Directory for file backups (relative to workspace)").
				Placeholder(".fest-backup").
				Value(&cfg.Local.BackupDir),
			huh.NewInput().
				Title("Checksum File").
				Description("File for tracking template checksums").
				Placeholder(".fest-checksums.json").
				Value(&cfg.Local.ChecksumFile),
		),
	)

	if err := uitheme.RunForm(ctx, form); err != nil {
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

// resetToDefaults resets configuration to default values
func resetToDefaults(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	confirmed, cancelled, err := uitheme.PromptConfirm(ctx, "Reset all settings to defaults?")
	if err != nil {
		return err
	}
	if cancelled || !confirmed {
		return nil
	}

	defaults := config.DefaultConfig()
	if err := config.Save(ctx, defaults); err != nil {
		return errors.Wrap(err, "saving default config")
	}

	display := ui.New(false, false)
	display.Success("Configuration reset to defaults")

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
