package theme

import (
	"strings"
	"testing"
)

// Ensure focused boxes use a full rounded border so the right edge is painted.
// Regression: BorderStyle alone without enabling sides / Width without frame
// reserve left the right border missing in create TUI demos.
func TestFocusedBaseHasAllBorderSides(t *testing.T) {
	t.Parallel()
	th := GetTheme(ThemeDark)
	base := th.Focused.Base

	if !base.GetBorderTop() || !base.GetBorderRight() || !base.GetBorderBottom() || !base.GetBorderLeft() {
		t.Fatalf("Focused.Base missing border sides: top=%v right=%v bottom=%v left=%v",
			base.GetBorderTop(), base.GetBorderRight(), base.GetBorderBottom(), base.GetBorderLeft())
	}

	// Render a fixed-width field and assert the right border glyph is present
	// on each content line (not clipped by width math).
	content := "Create what?\n> Festival\n  Phase"
	out := base.Width(40).Render(content)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected multi-line render, got %q", out)
	}
	// Rounded right border runes (╮  │  ╯) or normal (┐ │ ┘)
	for i, line := range lines {
		// strip ANSI for glyph checks
		plain := stripANSI(line)
		if plain == "" {
			continue
		}
		last, _ := lastRune(plain)
		switch i {
		case 0:
			if last != '╮' && last != '┐' && last != '╗' {
				t.Fatalf("top line missing right corner, last=%q line=%q", string(last), plain)
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
}

func TestFocusedBaseContentSizedKeepsRightBorder(t *testing.T) {
	t.Parallel()
	th := GetTheme(ThemeDark)
	// Content-sized (no Width): right border must remain. Forcing Width(N) on a
	// full-border style is what clips ╮/│/╯ — do not use form.WithWidth(term).
	out := th.Focused.Base.Render("Festival\nStandalone workflow\nPhase")
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected multi-line box, got %q", out)
	}
	for i, line := range lines {
		plain := stripANSI(line)
		last, ok := lastRune(plain)
		if !ok {
			t.Fatalf("empty line %d", i)
		}
		switch i {
		case 0:
			if last != '╮' && last != '┐' {
				t.Fatalf("top missing right corner: %q", plain)
			}
		case len(lines) - 1:
			if last != '╯' && last != '┘' {
				t.Fatalf("bottom missing right corner: %q", plain)
			}
		default:
			if last != '│' {
				t.Fatalf("side missing right border: %q", plain)
			}
		}
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
