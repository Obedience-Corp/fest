package theme

import (
	"context"
	"errors"
	"os"
	"strconv"

	festErrors "github.com/Obedience-Corp/fest/internal/errors"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Error codes for theme package.
const ErrCodeCancelled = "CANCELLED"

const (
	resetModeAutoWrap = "\x1b[?7l"
	setModeAutoWrap   = "\x1b[?7h"
)

// ErrUserCancelled is returned when user cancels with Ctrl-C or Esc.
var ErrUserCancelled = festErrors.New("operation cancelled by user").WithCode(ErrCodeCancelled)

// formFrameModel wraps a huh form so the chrome box is full-terminal-width and
// paints a complete rounded border.
//
// huh applies Base.Width(fieldWidth) then clips the group through a viewport
// MaxWidth(fieldWidth). lipgloss paints left/right borders *outside* Width, so
// field-level Focused.Base boxes always lose ╮/│/╯ when expanded to the
// terminal. Content-sized (width 0) boxes stay complete but look tiny and
// non-responsive. Instead: fields stay borderless; this model draws one
// responsive outer frame around the form View.
type formFrameModel struct {
	form        *huh.Form
	frame       lipgloss.Style
	width       int
	height      int
	frameBorder int // horizontal border cells (left+right), usually 2
}

func (m *formFrameModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *formFrameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Inner form fills the frame content area (total width − borders).
		// Frame uses Width(term − borders) so painted size == term (borders sit
		// outside lipgloss Width — see FormFrameStyle / lipgloss docs).
		inner := msg.Width - m.frameBorder
		if inner < 20 {
			inner = 20
		}
		m.form = m.form.WithWidth(inner)
		// Forward a height-only size msg so huh can still size viewports; width
		// is already fixed via WithWidth (huh ignores WindowSize width when set).
		msg.Width = 0
		f, cmd := m.form.Update(msg)
		m.form = f.(*huh.Form)
		return m, cmd
	}

	f, cmd := m.form.Update(msg)
	m.form = f.(*huh.Form)
	return m, cmd
}

func (m *formFrameModel) View() string {
	inner := m.form.View()
	if inner == "" {
		return ""
	}
	if m.width <= 0 {
		return m.frame.Render(inner)
	}
	// painted width = Width + horizontal borders; target full terminal.
	boxW := m.width - m.frameBorder
	if boxW < 10 {
		boxW = 10
	}
	return m.frame.Width(boxW).Render(inner)
}

// RunForm executes a form with the fest theme, context propagation, and proper error handling.
// Context cancellation will interrupt the form. Returns ErrUserCancelled if the user aborts.
// Keyboard interrupt (Ctrl-C) and Escape are enabled for clean exits.
// Theme is loaded from user config (~/.obey/fest/config.json).
//
// Layout: a full-terminal-width rounded frame is drawn around the form so boxes
// stay responsive and solid. Field-level Base borders are intentionally off —
// huh's group viewport would clip them (see formFrameModel).
func RunForm(ctx context.Context, form *huh.Form) error {
	if err := ctx.Err(); err != nil {
		return festErrors.Wrap(err, "context cancelled").WithOp("RunForm")
	}

	th := GetThemeFromConfig(ctx)
	form = form.
		WithTheme(th).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithShowHelp(true)

	// Preserve huh's line-oriented accessible runner. NewForm enables it for
	// TERM=dumb, but the custom Bubble Tea frame model below would otherwise
	// bypass that state along with the form's configured input and output.
	if os.Getenv("TERM") == "dumb" {
		return normalizeFormRunError(ctx, form.WithAccessible(true).RunWithContext(ctx))
	}

	// Match huh.Form.run: quit on submit, interrupt on cancel.
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	frame := FormFrameStyle(th)
	m := &formFrameModel{
		form:        form,
		frame:       frame,
		frameBorder: frame.GetHorizontalBorderSize(),
	}
	if m.frameBorder <= 0 {
		m.frameBorder = 2
	}
	// Seed width before the first WindowSizeMsg so the first paint is full-width
	// (PTY/VHS/some hosts deliver size late; without this, content-hug frames flash).
	if w := detectTermCols(); w > 0 {
		m.width = w
		inner := w - m.frameBorder
		if inner < 20 {
			inner = 20
		}
		m.form = m.form.WithWidth(inner)
	}

	opts := []tea.ProgramOption{
		tea.WithOutput(os.Stderr),
		tea.WithContext(ctx),
		tea.WithReportFocus(),
	}

	// The outer border intentionally occupies the full terminal width. Disable
	// terminal autowrap while Bubble Tea paints it so a glyph in the last column
	// does not advance the cursor and leave orphan border rows on repaint.
	_, _ = os.Stderr.WriteString(resetModeAutoWrap)
	defer func() { _, _ = os.Stderr.WriteString(setModeAutoWrap) }()

	final, err := tea.NewProgram(m, opts...).Run()
	if err := normalizeFormRunError(ctx, err); err != nil {
		return err
	}

	if final != nil {
		if bm, ok := final.(*formFrameModel); ok {
			switch bm.form.State {
			case huh.StateAborted:
				return ErrUserCancelled
			}
		}
	}
	return nil
}

func normalizeFormRunError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, huh.ErrUserAborted) {
		return ErrUserCancelled
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return festErrors.Wrap(err, "context cancelled").WithOp("RunForm")
	}
	return festErrors.Wrap(err, "form error").WithOp("RunForm")
}

// FormFrameStyle returns the full-width chrome box used by RunForm.
// Exported for tests that assert solid rounded corners at a target width.
func FormFrameStyle(th *huh.Theme) lipgloss.Style {
	borderColor := th.Focused.Base.GetBorderTopForeground()
	// Fallback if theme Base no longer carries BorderForeground.
	if borderColor == nil {
		borderColor = lipgloss.Color("238")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
}

// detectTermCols returns the best-effort terminal width for initial layout.
// Prefer the live TTY size; fall back to COLUMNS for scripted/VHS hosts.
func detectTermCols() int {
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w >= 40 {
		return w
	}
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w >= 40 {
		return w
	}
	if c := os.Getenv("COLUMNS"); c != "" {
		if w, err := strconv.Atoi(c); err == nil && w >= 40 {
			return w
		}
	}
	return 0
}

// RenderFormFrame paints content inside a form frame sized for termCols.
// painted width equals termCols when termCols >= 4.
func RenderFormFrame(th *huh.Theme, termCols int, content string) string {
	if content == "" {
		return ""
	}
	frame := FormFrameStyle(th)
	border := frame.GetHorizontalBorderSize()
	if border <= 0 {
		border = 2
	}
	boxW := termCols - border
	if boxW < 1 {
		boxW = 1
	}
	return frame.Width(boxW).Render(content)
}

// IsCancelled checks if an error indicates user or context cancellation.
// Returns true for ErrUserCancelled, huh.ErrUserAborted, and context cancellation.
func IsCancelled(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrUserCancelled) ||
		errors.Is(err, huh.ErrUserAborted) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		festErrors.Is(err, ErrCodeCancelled)
}

// PromptInput is a convenience function for single-input prompts.
// Returns the value, whether it was skipped (empty or cancelled), and any error.
func PromptInput(ctx context.Context, title, placeholder string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, festErrors.Wrap(err, "context cancelled")
	}

	var value string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				Placeholder(placeholder).
				Value(&value),
		),
	)

	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return "", true, nil
		}
		return "", false, err
	}

	return value, value == "", nil
}

// PromptSelect is a convenience function for single-select prompts.
// Returns the selected value, whether it was cancelled, and any error.
func PromptSelect[T comparable](ctx context.Context, title string, options []huh.Option[T]) (T, bool, error) {
	var value T
	if err := ctx.Err(); err != nil {
		return value, false, festErrors.Wrap(err, "context cancelled")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().
				Title(title).
				Options(options...).
				Value(&value),
		),
	)

	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return value, true, nil
		}
		return value, false, err
	}

	return value, false, nil
}

// PromptConfirm is a convenience function for yes/no prompts.
// Returns the boolean result, whether it was cancelled, and any error.
func PromptConfirm(ctx context.Context, title string) (bool, bool, error) {
	if err := ctx.Err(); err != nil {
		return false, false, festErrors.Wrap(err, "context cancelled")
	}

	var value bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Value(&value),
		),
	)

	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return false, true, nil
		}
		return false, false, err
	}

	return value, false, nil
}

// ToOptions converts a slice of strings to huh options.
func ToOptions(values []string) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(values))
	for _, v := range values {
		opts = append(opts, huh.NewOption(v, v))
	}
	return opts
}

// QuickInput runs a simple text input form with proper keyboard handling.
// Returns the value, whether cancelled, and any error.
func QuickInput(ctx context.Context, title, placeholder string, value *string) (bool, error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				Placeholder(placeholder).
				Value(value),
		),
	)
	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// QuickInputValidate runs a simple text input form with validation.
// Returns whether cancelled, and any error.
func QuickInputValidate(ctx context.Context, title, placeholder string, value *string, validate func(string) error) (bool, error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				Placeholder(placeholder).
				Value(value).
				Validate(validate),
		),
	)
	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// QuickSelect runs a simple select form with proper keyboard handling.
// Returns whether cancelled, and any error.
func QuickSelect[T comparable](ctx context.Context, title string, options []huh.Option[T], value *T) (bool, error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().
				Title(title).
				Description("j/k or ↑/↓ navigate • enter select • esc quit").
				Options(options...).
				Value(value),
		),
	)
	if err := RunForm(ctx, form); err != nil {
		if IsCancelled(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
