// Package ui provides UI components and styling for the fest CLI.
//
// The shared brand palette is the source of truth for terminal colors. This
// file adapts its semantic roles to fest's older status/entity API so existing
// callers can migrate without a flag day.
package ui

import (
	"context"
	"os"
	"sync"

	"github.com/Obedience-Corp/fest/internal/config"
	sharedbrand "github.com/Obedience-Corp/obey-shared/brand"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

var (
	currentPalette      Palette
	currentBrandPalette = defaultSharedDarkPalette()
	configuredMode      = sharedbrand.ModeDark
	paletteOnce         sync.Once
	paletteInit         bool
)

// Palette holds the colors used throughout the CLI. Its fields are kept for
// compatibility with existing fest consumers; values are resolved from the
// shared semantic brand roles at startup.
type Palette struct {
	// Status colors for festival states
	Active    lipgloss.TerminalColor
	Ready     lipgloss.TerminalColor
	Planning  lipgloss.TerminalColor
	Ritual    lipgloss.TerminalColor
	Completed lipgloss.TerminalColor
	Archived  lipgloss.TerminalColor
	Someday   lipgloss.TerminalColor
	Dungeon   lipgloss.TerminalColor

	// Entity colors
	Festival lipgloss.TerminalColor
	Phase    lipgloss.TerminalColor
	Sequence lipgloss.TerminalColor
	Task     lipgloss.TerminalColor
	Gate     lipgloss.TerminalColor

	// State colors
	Pending    lipgloss.TerminalColor
	InProgress lipgloss.TerminalColor
	Blocked    lipgloss.TerminalColor
	Success    lipgloss.TerminalColor
	Warning    lipgloss.TerminalColor
	Error      lipgloss.TerminalColor

	// Structural colors
	Border   lipgloss.TerminalColor
	Value    lipgloss.TerminalColor
	Metadata lipgloss.TerminalColor
	Dim      lipgloss.TerminalColor
}

// InitPalette loads the configured shared brand palette.
// Must be called early in main() before any UI output.
func InitPalette(ctx context.Context) {
	paletteOnce.Do(func() {
		cfg, err := config.Load(ctx, "")
		if err != nil {
			configuredMode = sharedbrand.ModeDark
			setPalette(ResolveBrandPalette(configuredMode))
			return
		}

		configuredMode = sharedbrand.ParseMode(cfg.TUI.Theme)
		setPalette(ResolveBrandPalette(configuredMode))
	})
}

func setPalette(p sharedbrand.Palette) {
	currentBrandPalette = p
	currentPalette = paletteFromShared(p)
	paletteInit = true
	syncLegacyColors(currentPalette)

	// Keep markdown and other background-sensitive renderers aligned with the
	// selected explicit mode. Adaptive has already been resolved by brand.
	switch p.Mode {
	case sharedbrand.ModeLight:
		lipgloss.SetHasDarkBackground(false)
	case sharedbrand.ModeDark, sharedbrand.ModeHighContrast:
		lipgloss.SetHasDarkBackground(true)
	}
}

func syncLegacyColors(p Palette) {
	ActiveColor, ReadyColor, PlanningColor = paletteLipglossColor(p.Active), paletteLipglossColor(p.Ready), paletteLipglossColor(p.Planning)
	RitualColor, CompletedColor = paletteLipglossColor(p.Ritual), paletteLipglossColor(p.Completed)
	ArchivedColor, SomedayColor, DungeonColor = paletteLipglossColor(p.Archived), paletteLipglossColor(p.Someday), paletteLipglossColor(p.Dungeon)
	FestivalColor, PhaseColor = paletteLipglossColor(p.Festival), paletteLipglossColor(p.Phase)
	SequenceColor, TaskColor, GateColor = paletteLipglossColor(p.Sequence), paletteLipglossColor(p.Task), paletteLipglossColor(p.Gate)
	PendingColor, InProgressColor = paletteLipglossColor(p.Pending), paletteLipglossColor(p.InProgress)
	JudgeColor = paletteLipglossColor(p.Task)
	BlockedColor, SuccessColor = paletteLipglossColor(p.Blocked), paletteLipglossColor(p.Success)
	WarningColor, ErrorColor = paletteLipglossColor(p.Warning), paletteLipglossColor(p.Error)
	BorderColor, ValueColor = paletteLipglossColor(p.Border), paletteLipglossColor(p.Value)
	MetadataColor = paletteLipglossColor(p.Metadata)

	WorkTypeImplColor = paletteLipglossColor(p.Success)
	WorkTypeAnalysisColor = paletteLipglossColor(p.Festival)
	WorkTypeReviewColor = paletteLipglossColor(p.Planning)
	WorkTypeVerifyColor = paletteLipglossColor(p.Warning)
	WorkTypeConfigColor = paletteLipglossColor(p.Metadata)
	WorkTypeDocsColor = paletteLipglossColor(p.InProgress)
	WorkTypePlanColor = paletteLipglossColor(p.Task)
	WorkTypeActionColor = paletteLipglossColor(p.Sequence)

	ActiveStyle = lipgloss.NewStyle().Foreground(ActiveColor).Bold(true)
	ReadyStyle = lipgloss.NewStyle().Foreground(ReadyColor).Bold(true)
	PlanningStyle = lipgloss.NewStyle().Foreground(PlanningColor).Bold(true)
	RitualStyle = lipgloss.NewStyle().Foreground(RitualColor).Bold(true)
	CompletedStyle = lipgloss.NewStyle().Foreground(CompletedColor).Bold(true)
	ArchivedStyle = lipgloss.NewStyle().Foreground(ArchivedColor).Bold(true)
	SomedayStyle = lipgloss.NewStyle().Foreground(SomedayColor).Bold(true)
	DungeonStyle = lipgloss.NewStyle().Foreground(DungeonColor).Bold(true)
}

// ResolveBrandPalette resolves a shared semantic palette using fest's current
// output capabilities. It never queries the terminal background; bginit seeds
// Lip Gloss before command initialization.
func ResolveBrandPalette(mode sharedbrand.Mode) sharedbrand.Palette {
	profile := termenv.EnvColorProfile()
	caps := sharedbrand.EnvironmentCapabilities(outputIsTTY(), colorDepth(profile))
	caps.DarkBackground = lipgloss.HasDarkBackground()
	caps.BackgroundKnown = true
	if noColorOverride {
		caps.NoColor = true
		caps.ColorDepth = sharedbrand.ColorNone
	}
	return sharedbrand.Resolve(mode, caps)
}

// CurrentBrandPalette returns the resolved shared palette. Before command
// initialization it returns an interactive dark fallback for package users
// and tests that render directly without Cobra's lifecycle.
func CurrentBrandPalette() sharedbrand.Palette {
	if !paletteInit {
		return defaultSharedDarkPalette()
	}
	return currentBrandPalette
}

// InteractivePalette resolves the configured theme for a TUI rendered to a
// terminal other than stdout. Shell navigation wrappers capture stdout so
// they can use it as the selected path, while the picker itself renders to
// stderr. In that case Current() is intentionally plain, but the TUI still
// has a color-capable terminal to render into.
func InteractivePalette() Palette {
	if noColorOverride || configuredMode == sharedbrand.ModePlain {
		return Palette{}
	}

	profile := termenv.EnvColorProfile()
	shared := sharedbrand.Resolve(configuredMode, sharedbrand.Capabilities{
		IsTTY:           true,
		ColorDepth:      colorDepth(profile),
		DarkBackground:  lipgloss.HasDarkBackground(),
		BackgroundKnown: true,
	})
	return paletteFromShared(shared)
}

// completionPalette resolves the configured theme for explicit shell
// completion display colors. Completion commands write into a shell pipe, so
// the normal output policy resolves their palette to plain; the shell itself
// still supports ANSI display cells and requested these colors explicitly.
func completionPalette() Palette {
	if noColorOverride || configuredMode == sharedbrand.ModePlain {
		return Palette{}
	}

	shared := sharedbrand.Resolve(configuredMode, sharedbrand.Capabilities{
		IsTTY:           true,
		ColorDepth:      sharedbrand.ColorANSI256,
		DarkBackground:  lipgloss.HasDarkBackground(),
		BackgroundKnown: true,
	})
	return paletteFromShared(shared)
}

func outputIsTTY() bool {
	file, ok := termenv.DefaultOutput().Writer().(*os.File)
	if !ok {
		return false
	}
	fd := int(file.Fd())
	return term.IsTerminal(fd)
}

func colorDepth(profile termenv.Profile) sharedbrand.ColorDepth {
	switch profile {
	case termenv.TrueColor:
		return sharedbrand.ColorTrueColor
	case termenv.ANSI256:
		return sharedbrand.ColorANSI256
	case termenv.ANSI:
		return sharedbrand.ColorANSI16
	default:
		return sharedbrand.ColorNone
	}
}

func defaultSharedDarkPalette() sharedbrand.Palette {
	return sharedbrand.Resolve(sharedbrand.ModeDark, sharedbrand.Capabilities{
		IsTTY:           true,
		ColorDepth:      sharedbrand.ColorTrueColor,
		DarkBackground:  true,
		BackgroundKnown: true,
	})
}

func paletteColor(value string) lipgloss.TerminalColor {
	return lipgloss.Color(value)
}

func paletteLipglossColor(color lipgloss.TerminalColor) lipgloss.Color {
	value, _ := color.(lipgloss.Color)
	return value
}

// paletteFromShared maps fest-specific concepts onto shared semantic roles.
// Statuses and workflow states use status roles; hierarchy uses the brand
// accent family to stay distinct without inventing another color system.
func paletteFromShared(p sharedbrand.Palette) Palette {
	planning, ritual, completed := festivalStatusColors(p)
	task := festivalTaskColor(p)

	return Palette{
		Active:    paletteColor(p.StatusSuccess),
		Ready:     paletteColor(p.StatusWarning),
		Planning:  paletteColor(planning),
		Ritual:    paletteColor(ritual),
		Completed: paletteColor(completed),
		Archived:  paletteColor(p.TextMuted),
		Someday:   paletteColor(p.AccentSubtle),
		Dungeon:   paletteColor(p.TextMuted),

		Festival: paletteColor(p.Accent),
		Phase:    paletteColor(p.AccentHighlight),
		Sequence: paletteColor(p.AccentStrong),
		Task:     paletteColor(task),
		Gate:     paletteColor(p.StatusWarning),

		Pending:    paletteColor(p.TextMuted),
		InProgress: paletteColor(p.StatusWarning),
		Blocked:    paletteColor(p.StatusError),
		Success:    paletteColor(p.StatusSuccess),
		Warning:    paletteColor(p.StatusWarning),
		Error:      paletteColor(p.StatusError),

		Border:   paletteColor(p.Border),
		Value:    paletteColor(p.TextPrimary),
		Metadata: paletteColor(p.TextMuted),
		Dim:      paletteColor(p.TextMuted),
	}
}

// festivalStatusColors preserves the established lifecycle color distinctions
// that are more granular than the shared brand's generic accent roles. In
// particular, planning must remain blue, ritual must remain purple, and
// completed must remain magenta so those statuses do not collapse into the
// shared orange family or active green.
func festivalStatusColors(p sharedbrand.Palette) (planning, ritual, completed string) {
	if !p.ColorEnabled || p.Mode == sharedbrand.ModePlain {
		return "", "", ""
	}

	switch p.Mode {
	case sharedbrand.ModeLight:
		return "#1D4ED8", "#7C3AED", "#A21CAF"
	case sharedbrand.ModeHighContrast:
		return "#60A5FA", "#D8B4FE", "#FF79C6"
	default:
		return "#4DA3FF", "#B388FF", "#FF79C6"
	}
}

// festivalTaskColor keeps task names in the established purple family. Task
// names are hierarchy labels, not error indicators, so they must not inherit
// the shared brand's reddish AccentSubtle role.
func festivalTaskColor(p sharedbrand.Palette) string {
	if !p.ColorEnabled || p.Mode == sharedbrand.ModePlain {
		return ""
	}

	switch p.Mode {
	case sharedbrand.ModeLight:
		return "#7C3AED"
	case sharedbrand.ModeHighContrast:
		return "#D8B4FE"
	default:
		return "#B388FF"
	}
}

// Current returns the current palette.
// If InitPalette hasn't been called, returns the shared dark fallback.
func Current() *Palette {
	if !paletteInit {
		return defaultDarkPalettePtr()
	}
	return &currentPalette
}

func defaultDarkPalette() Palette {
	return paletteFromShared(defaultSharedDarkPalette())
}

var defaultDarkPaletteInstance *Palette

func defaultDarkPalettePtr() *Palette {
	if defaultDarkPaletteInstance == nil {
		p := defaultDarkPalette()
		defaultDarkPaletteInstance = &p
	}
	return defaultDarkPaletteInstance
}

// ResetPalette resets the palette state (for testing only).
func ResetPalette() {
	paletteOnce = sync.Once{}
	paletteInit = false
	currentPalette = Palette{}
	currentBrandPalette = defaultSharedDarkPalette()
	configuredMode = sharedbrand.ModeDark
	defaultDarkPaletteInstance = nil
	syncLegacyColors(defaultDarkPalette())
}
