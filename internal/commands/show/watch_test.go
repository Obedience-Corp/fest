package show

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type limitedWriter struct {
	limit int
	buf   bytes.Buffer
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > l.limit {
		n = l.limit
	}
	l.buf.Write(p[:n])
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

func TestCRLFWriter_TranslatesBareNewlines(t *testing.T) {
	var buf bytes.Buffer
	w := crlfWriter{w: &buf}

	in := "line1\nline2\n"
	n, err := w.Write([]byte(in))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(in) {
		t.Errorf("Write returned %d, want original length %d", n, len(in))
	}
	if got, want := buf.String(), "line1\r\nline2\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCRLFWriter_DoesNotDoubleExistingCRLF(t *testing.T) {
	var buf bytes.Buffer
	w := crlfWriter{w: &buf}

	if _, err := w.Write([]byte("a\r\nb\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "a\r\nb\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCRLFWriter_LeavesEscapeSequencesIntact(t *testing.T) {
	var buf bytes.Buffer
	w := crlfWriter{w: &buf}

	if _, err := w.Write([]byte("\033[H\033[2J")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := buf.String(), "\033[H\033[2J"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWatchWriter_CycleModeWrapsCRLF(t *testing.T) {
	if _, ok := watchWriter(false).(crlfWriter); ok {
		t.Error("watchWriter(false) should not translate newlines")
	}
	if _, ok := watchWriter(true).(crlfWriter); !ok {
		t.Error("watchWriter(true) should translate newlines (raw mode)")
	}
}

func TestWatchErrWriter_CycleModeWrapsCRLF(t *testing.T) {
	if _, ok := watchErrWriter(false).(crlfWriter); ok {
		t.Error("watchErrWriter(false) should not translate newlines")
	}
	if _, ok := watchErrWriter(true).(crlfWriter); !ok {
		t.Error("watchErrWriter(true) should translate newlines (raw mode)")
	}
}

func TestCRLFWriter_PreservesPartialWriteCount(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		limit   int
		wantN   int
		wantErr bool
	}{
		{"stops mid-crlf pair", "ab\ncd", 3, 2, true},
		{"stops after translated newline", "ab\ncd", 4, 3, true},
		{"full write", "ab\ncd", 6, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &limitedWriter{limit: tt.limit}
			n, err := crlfWriter{w: lw}.Write([]byte(tt.in))
			if n != tt.wantN {
				t.Errorf("Write returned n=%d, want %d", n, tt.wantN)
			}
			if tt.wantErr && err == nil {
				t.Error("expected a short-write error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if n < 0 || n > len(tt.in) {
				t.Errorf("n=%d violates io.Writer contract for input length %d", n, len(tt.in))
			}
		})
	}
}

func TestPrintWatchFooter_CycleModeEmitsCRLF(t *testing.T) {
	var buf bytes.Buffer
	session := newWatchFrameSession(crlfWriter{w: &buf}, nil, true, true, true, false)
	session.printFooter(0, 0, 0, false)

	out := buf.String()
	if !strings.Contains(out, "\r\n") {
		t.Errorf("cycle-mode footer should use CRLF line endings, got %q", out)
	}
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Errorf("cycle-mode footer has a bare \\n (cursor would not return to column 0): %q", out)
	}
}

func TestPrintWatchFooter_PromoteHint(t *testing.T) {
	cases := []struct {
		name        string
		cycleHint   bool
		cycling     bool
		promoteHint bool
		wantPromote bool
		wantCycle   bool
		wantQExit   bool
	}{
		{"watch_raw_single", true, false, true, true, false, true},
		{"watch_raw_multi", true, true, true, true, true, true},
		{"show_raw_single", true, false, false, false, false, true},
		{"show_raw_multi", true, true, false, false, true, true},
		{"non_raw", false, false, false, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			session := newWatchFrameSession(&buf, nil, tc.cycleHint, tc.cycling, tc.promoteHint, false)
			session.printFooter(0, 0, 0, false)
			out := buf.String()
			if got := strings.Contains(out, "p promote"); got != tc.wantPromote {
				t.Errorf("promote hint present = %v, want %v (footer %q)", got, tc.wantPromote, out)
			}
			if got := strings.Contains(out, "cycle"); got != tc.wantCycle {
				t.Errorf("cycle hint present = %v, want %v (footer %q)", got, tc.wantCycle, out)
			}
			if got := strings.Contains(out, "q/Ctrl+C exit"); got != tc.wantQExit {
				t.Errorf("q/Ctrl+C exit hint present = %v, want %v (footer %q)", got, tc.wantQExit, out)
			}
			if !tc.cycleHint && !strings.Contains(out, "Press Ctrl+C to exit") {
				t.Errorf("non-raw footer should keep the plain exit hint, got %q", out)
			}
		})
	}
}

func TestRenderFestivalView_TreeShowsLifecycleStatus(t *testing.T) {
	festDir := setupTempFestival(t)
	festival := &FestivalInfo{Name: "test-fest", Status: "active", Path: festDir}

	var buf bytes.Buffer
	if err := renderFestivalView(t.Context(), festival, &showOptions{}, &buf); err != nil {
		t.Fatalf("renderFestivalView: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Status") || !strings.Contains(out, "active") {
		t.Errorf("tree watch view should surface the lifecycle status, got %q", out)
	}
	if strings.Contains(out, "Path") {
		t.Errorf("expected tree view, but summary fallback rendered: %q", out)
	}
}
