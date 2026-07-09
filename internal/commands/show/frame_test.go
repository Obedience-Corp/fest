package show

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func contentLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%03d", i+1)
	}
	return lines
}

func TestFrameStateViewportShortContentNoOverflow(t *testing.T) {
	f := NewFrameState()
	lines := contentLines(5)

	visible, from, to, total, overflow := f.Viewport(lines, 10)
	if overflow {
		t.Fatal("content shorter than viewport must not report overflow")
	}
	if len(visible) != 5 || from != 1 || to != 5 || total != 5 {
		t.Fatalf("viewport = (%d lines, %d-%d/%d)", len(visible), from, to, total)
	}
}

func TestFrameStateScrollLinesClampsToContent(t *testing.T) {
	f := NewFrameState()
	lines := contentLines(30)

	f.Viewport(lines, 10)
	f.ScrollLines(5)
	visible, from, to, _, overflow := f.Viewport(lines, 10)
	if !overflow {
		t.Fatal("30 lines in a 10-line viewport must overflow")
	}
	if from != 6 || to != 15 || visible[0] != "line-006" {
		t.Fatalf("after +5: %d-%d, first %q", from, to, visible[0])
	}

	f.ScrollLines(1000)
	_, from, to, _, _ = f.Viewport(lines, 10)
	if from != 21 || to != 30 {
		t.Fatalf("over-scroll must clamp to bottom, got %d-%d", from, to)
	}

	f.ScrollLines(-1000)
	_, from, to, _, _ = f.Viewport(lines, 10)
	if from != 1 || to != 10 {
		t.Fatalf("under-scroll must clamp to top, got %d-%d", from, to)
	}
}

func TestFrameStateScrollPagesUsesViewportSize(t *testing.T) {
	f := NewFrameState()
	lines := contentLines(40)

	f.Viewport(lines, 10)
	f.ScrollPages(1)
	_, from, _, _, _ := f.Viewport(lines, 10)
	if from != 11 {
		t.Fatalf("page down from top: from = %d, want 11", from)
	}

	f.ScrollPages(-1)
	_, from, _, _, _ = f.Viewport(lines, 10)
	if from != 1 {
		t.Fatalf("page up back to top: from = %d, want 1", from)
	}
}

func TestFrameStateScrollSignalsRedrawOnceAndOnlyOnChange(t *testing.T) {
	f := NewFrameState()
	f.Viewport(contentLines(30), 10)

	f.ScrollLines(1)
	select {
	case <-f.Redraw():
	default:
		t.Fatal("offset change must signal redraw")
	}

	f.ScrollLines(-1000)
	select {
	case <-f.Redraw():
	default:
	}

	f.ScrollLines(-1)
	select {
	case <-f.Redraw():
		t.Fatal("scroll at clamp boundary must not signal redraw")
	default:
	}
}

func TestFrameStateResetReturnsToTop(t *testing.T) {
	f := NewFrameState()
	lines := contentLines(30)
	f.Viewport(lines, 10)
	f.ScrollLines(15)
	f.Reset()

	_, from, _, _, _ := f.Viewport(lines, 10)
	if from != 1 {
		t.Fatalf("after Reset: from = %d, want 1", from)
	}
}

func TestWatchFrameSessionDrawSlicesViewport(t *testing.T) {
	var buf bytes.Buffer
	frame := NewFrameState()
	session := newWatchFrameSession(&buf, frame, true, true, false, false)
	session.height = func() int { return 12 }

	session.setContent(strings.Join(contentLines(30), "\n") + "\n")
	out := buf.String()
	if !strings.Contains(out, "line-001") || !strings.Contains(out, "line-010") {
		t.Fatalf("viewport should show first 10 lines, got:\n%s", out)
	}
	if strings.Contains(out, "line-011") {
		t.Fatalf("line past the viewport leaked into the frame:\n%s", out)
	}
	if !strings.Contains(out, "scroll (1-10/30)") {
		t.Fatalf("footer missing scroll indicator, got:\n%s", out)
	}

	buf.Reset()
	frame.ScrollLines(5)
	session.draw()
	out = buf.String()
	if !strings.Contains(out, "line-006") || strings.Contains(out, "line-005") {
		t.Fatalf("scrolled viewport should start at line-006:\n%s", out)
	}
	if !strings.Contains(out, "scroll (6-15/30)") {
		t.Fatalf("footer indicator not updated after scroll:\n%s", out)
	}
}

func TestWatchFrameSessionNilFrameShowsFullContent(t *testing.T) {
	var buf bytes.Buffer
	session := newWatchFrameSession(&buf, nil, false, false, false, false)
	session.height = func() int { return 12 }

	session.setContent(strings.Join(contentLines(30), "\n") + "\n")
	out := buf.String()
	if !strings.Contains(out, "line-001") || !strings.Contains(out, "line-030") {
		t.Fatalf("nil frame must print full content unchanged:\n%s", out)
	}
	if strings.Contains(out, "scroll (") {
		t.Fatalf("nil frame must not show a scroll indicator:\n%s", out)
	}
}

func TestWatchFrameSessionUnknownHeightShowsFullContent(t *testing.T) {
	var buf bytes.Buffer
	frame := NewFrameState()
	session := newWatchFrameSession(&buf, frame, true, true, false, false)
	session.height = func() int { return 0 }

	session.setContent(strings.Join(contentLines(30), "\n") + "\n")
	out := buf.String()
	if !strings.Contains(out, "line-030") {
		t.Fatalf("unknown terminal size must fall back to full content:\n%s", out)
	}
}
