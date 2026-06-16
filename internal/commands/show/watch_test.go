package show

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestPrintWatchFooter_CycleModeEmitsCRLF(t *testing.T) {
	var buf bytes.Buffer
	printWatchFooter(crlfWriter{w: &buf}, false, true)

	out := buf.String()
	if !strings.Contains(out, "\r\n") {
		t.Errorf("cycle-mode footer should use CRLF line endings, got %q", out)
	}
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Errorf("cycle-mode footer has a bare \\n (cursor would not return to column 0): %q", out)
	}
}
