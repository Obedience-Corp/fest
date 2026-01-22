//go:build !no_charm

package config

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/spf13/cobra"
)

// NewThemeCommand creates the theme command group.
func NewThemeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "Manage TUI color theme",
		Long: `Manage the TUI color theme for fest commands.

Available themes:
  adaptive      Auto-detect light/dark terminal (default)
  light         Optimized for light backgrounds
  dark          Optimized for dark backgrounds
  high-contrast Pure white + bright colors (works on any background)

Use 'fest config theme test' to preview all themes on your terminal.`,
	}

	cmd.AddCommand(newThemeShowCommand())
	cmd.AddCommand(newThemeSetCommand())
	cmd.AddCommand(newThemeTestCommand())

	return cmd
}

func newThemeShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current theme setting",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemeShow(cmd.Context())
		},
	}
}

func runThemeShow(ctx context.Context) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	cfg, err := config.Load(ctx, "")
	if err != nil {
		return errors.Wrap(err, "loading config")
	}

	currentTheme := cfg.TUI.Theme
	if currentTheme == "" {
		currentTheme = "adaptive"
	}

	display.Info("Current theme: %s", currentTheme)
	display.Info("Valid themes: %v", theme.ValidThemes())
	return nil
}

func newThemeSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <theme>",
		Short: "Set the TUI theme",
		Long: `Set the TUI color theme.

Available themes:
  adaptive      Auto-detect light/dark terminal (default)
  light         Optimized for light backgrounds
  dark          Optimized for dark backgrounds
  high-contrast Pure white + bright colors (works on any background)`,
		Example: `  fest config theme set high-contrast
  fest config theme set dark`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemeSet(cmd.Context(), args[0])
		},
	}
}

func runThemeSet(ctx context.Context, themeName string) error {
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	if !theme.IsValidTheme(themeName) {
		return errors.Validation("invalid theme").
			WithField("theme", themeName).
			WithField("valid", theme.ValidThemes())
	}

	cfg, err := config.Load(ctx, "")
	if err != nil {
		return errors.Wrap(err, "loading config")
	}

	cfg.TUI.Theme = themeName

	if err := config.Save(ctx, cfg); err != nil {
		return errors.Wrap(err, "saving config")
	}

	display.Success("Theme set to '%s'", themeName)
	display.Info("Config saved to %s", config.ConfigDir())
	return nil
}

func newThemeTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Preview all themes side by side",
		Long: `Preview all available themes to see which works best on your terminal background.

This displays sample TUI elements with each theme so you can compare
how they look on your current terminal background color.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemeTest(cmd.Context())
		},
	}
}

func runThemeTest(ctx context.Context) error {
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render("Theme Comparison - Select Options Work Best On Your Background"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Use j/k or arrows to navigate, enter to confirm, esc to quit"))
	fmt.Println()

	themes := []string{"adaptive", "light", "dark", "high-contrast"}

	for _, themeName := range themes {
		printThemeSample(themeName)
	}

	// Interactive selection to let user pick one
	var selected string
	options := make([]huh.Option[string], len(themes))
	for i, t := range themes {
		options[i] = huh.NewOption(t, t)
	}

	// Use high-contrast theme for the selector itself (most visible)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a theme to apply").
				Description("The theme will be saved to your config").
				Options(options...).
				Value(&selected),
		),
	).WithTheme(theme.GetTheme(theme.ThemeHighContrast))

	if err := form.Run(); err != nil {
		// User cancelled
		fmt.Println("Selection cancelled - no changes made")
		return nil
	}

	// Apply the selected theme
	return runThemeSet(ctx, selected)
}

func printThemeSample(themeName string) {
	t := theme.GetTheme(theme.ThemeName(themeName))

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#444444")).
		Padding(0, 1)

	fmt.Println(headerStyle.Render(fmt.Sprintf("═══ %s ═══", themeName)))

	// Sample elements using the theme
	fmt.Printf("  %s\n", t.Focused.Title.Render("Title: Sample Form"))
	fmt.Printf("  %s\n", t.Focused.Description.Render("Description: Choose an option"))
	fmt.Printf("  %s%s\n", t.Focused.SelectSelector.String(), t.Focused.SelectedOption.Render("Selected Option"))
	fmt.Printf("    %s\n", t.Focused.Option.Render("Regular Option"))
	fmt.Printf("    %s\n", t.Focused.Option.Render("Another Option"))
	fmt.Printf("  %s\n", t.Focused.ErrorMessage.Render("Error: Something went wrong"))
	fmt.Println()
}
