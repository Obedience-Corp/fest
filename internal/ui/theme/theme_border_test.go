package theme

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func TestRunFormAccessibleTERM(t *testing.T) {
	if os.Getenv("FEST_TEST_ACCESSIBLE_FORM") == "1" {
		var selected string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose one").
				Options(huh.NewOption("Alpha", "alpha"), huh.NewOption("Beta", "beta")).
				Value(&selected),
		))
		if err := RunForm(context.Background(), form); err != nil {
			fmt.Fprintf(os.Stderr, "RunForm failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("selected=%s\n", selected)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunFormAccessibleTERM$")
	cmd.Env = append(os.Environ(), "FEST_TEST_ACCESSIBLE_FORM=1", "TERM=dumb")
	cmd.Stdin = strings.NewReader("1\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("accessible form subprocess failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "selected=alpha") {
		t.Fatalf("accessible form did not read piped input:\n%s", stdout.String())
	}
	if strings.Contains(stderr.String(), "could not open a new TTY") {
		t.Fatalf("accessible form tried to open a TTY:\n%s", stderr.String())
	}
}

func TestRenderFormFramePreservesEmptyFinalView(t *testing.T) {
	t.Parallel()
	if got := RenderFormFrame(GetTheme(ThemeDark), 80, ""); got != "" {
		t.Fatalf("empty final form view rendered stale frame: %q", got)
	}
}

// A finished form must render nothing. huh returns "" once the form is
// quitting; framing that empty string paints a two-line box that Bubble Tea
// prints as the program's final view. The create wizard runs one program per
// step, so every finished step used to leave a stale box in scrollback (3 above
// Project, 5 above Seed, 7 above Confirm).
//
// This exercises formFrameModel.View, the path RunForm actually uses;
// RenderFormFrame above is only reached from tests.
func TestFormFrameModelRendersNothingWhenFinished(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"submitted", tea.KeyMsg{Type: tea.KeyEnter}},
		{"aborted", tea.KeyMsg{Type: tea.KeyCtrlC}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newFrameModelForTest()
			if m.View() == "" {
				t.Fatal("form should paint a frame while it is still running")
			}
			pumpFrameModel(t, m, tc.key)
			if got := m.View(); got != "" {
				t.Fatalf("finished form left a frame in scrollback:\n%q", got)
			}
		})
	}
}

func newFrameModelForTest() *formFrameModel {
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Choose one").
			Options(huh.NewOption("Alpha", "alpha"), huh.NewOption("Beta", "beta")).
			Value(&selected),
	)).WithTheme(GetTheme(ThemeDark)).WithKeyMap(huh.NewDefaultKeyMap())
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	frame := FormFrameStyle(GetTheme(ThemeDark))
	border := frame.GetHorizontalBorderSize()
	if border <= 0 {
		border = 2
	}
	return &formFrameModel{form: form, frame: frame, frameBorder: border, width: 80}
}

// pumpFrameModel delivers msgs and the messages their commands produce, so the
// model reaches the same state it would inside a running Bubble Tea program.
func pumpFrameModel(t *testing.T, m *formFrameModel, msgs ...tea.Msg) {
	t.Helper()
	queue := append([]tea.Msg(nil), msgs...)
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 100 {
			t.Fatal("form update loop did not settle")
		}
		msg := queue[0]
		queue = queue[1:]
		_, cmd := m.Update(msg)
		queue = append(queue, drainCmd(t, cmd, 0)...)
	}
}

func drainCmd(t *testing.T, cmd tea.Cmd, depth int) []tea.Msg {
	t.Helper()
	if cmd == nil || depth > 8 {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, drainCmd(t, c, depth+1)...)
		}
		return out
	case tea.QuitMsg, tea.InterruptMsg:
		// Program-level signals; the runtime consumes these, not the model.
		return nil
	default:
		return []tea.Msg{msg}
	}
}

// Full-width form chrome must paint a complete rounded box at the terminal
// width. Regression: field-level Focused.Base borders were clipped by huh's
// group viewport; content-sized (width 0) boxes stayed complete but looked tiny.
func TestFormFrameFullWidthHasAllCorners(t *testing.T) {
	t.Parallel()
	th := GetTheme(ThemeDark)
	termCols := 80
	content := "Create what?\n> Festival\n  Phase\n  Sequence"
	out := RenderFormFrame(th, termCols, content)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected multi-line frame, got %q", out)
	}

	maxW := 0
	for i, line := range lines {
		plain := stripANSI(line)
		w := lipgloss.Width(plain)
		if w > maxW {
			maxW = w
		}
		if plain == "" {
			continue
		}
		last, ok := lastRune(plain)
		if !ok {
			t.Fatalf("empty line %d", i)
		}
		switch i {
		case 0:
			if last != '╮' && last != '┐' && last != '╗' {
				t.Fatalf("top line missing right corner, last=%q line=%q", string(last), plain)
			}
			if !strings.HasPrefix(plain, "╭") && !strings.HasPrefix(plain, "┌") {
				t.Fatalf("top line missing left corner: %q", plain)
			}
		case len(lines) - 1:
			if last != '╯' && last != '┘' && last != '╝' {
				t.Fatalf("bottom line missing right corner, last=%q line=%q", string(last), plain)
			}
		default:
			if last != '│' && last != '║' {
				t.Fatalf("mid line missing right border, last=%q line=%q", string(last), plain)
			}
		}
	}
	if maxW != termCols {
		t.Fatalf("frame should paint full terminal width: maxW=%d want %d\n%s", maxW, termCols, out)
	}
}

func TestFormFrameStyleHasAllBorderSides(t *testing.T) {
	t.Parallel()
	th := GetTheme(ThemeDark)
	frame := FormFrameStyle(th)
	if !frame.GetBorderTop() || !frame.GetBorderRight() || !frame.GetBorderBottom() || !frame.GetBorderLeft() {
		t.Fatalf("FormFrameStyle missing border sides: top=%v right=%v bottom=%v left=%v",
			frame.GetBorderTop(), frame.GetBorderRight(), frame.GetBorderBottom(), frame.GetBorderLeft())
	}
}

func TestFocusedBaseIsBorderless(t *testing.T) {
	t.Parallel()
	// Field Base must stay borderless so huh viewports do not clip a right edge.
	// Chrome lives on FormFrameStyle / RunForm.
	th := GetTheme(ThemeDark)
	base := th.Focused.Base
	if base.GetBorderTop() || base.GetBorderRight() || base.GetBorderBottom() || base.GetBorderLeft() {
		t.Fatalf("Focused.Base should be borderless (outer form frame owns chrome); sides top=%v right=%v bottom=%v left=%v",
			base.GetBorderTop(), base.GetBorderRight(), base.GetBorderBottom(), base.GetBorderLeft())
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func lastRune(s string) (rune, bool) {
	var last rune
	ok := false
	for _, r := range s {
		last = r
		ok = true
	}
	return last, ok
}
