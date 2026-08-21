package ui

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestConfirm_NonTTYReturnsFalseWithoutReading(t *testing.T) {
	orig := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = orig })
	stdinIsTerminal = func() bool { return false }

	u := &UI{noColor: true, reader: bufio.NewReader(strings.NewReader(""))}

	stdout := captureStdout(t, func() {
		if u.Confirm("Create task files now?") {
			t.Fatal("expected false when stdin is not a TTY")
		}
	})
	if stdout != "" {
		t.Fatalf("non-TTY Confirm must not print a prompt, got %q", stdout)
	}
}

func TestConfirm_TTYYes(t *testing.T) {
	orig := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = orig })
	stdinIsTerminal = func() bool { return true }

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "yes", in: "y\n", want: true},
		{name: "YES", in: "yes\n", want: true},
		{name: "empty default yes", in: "\n", want: true},
		{name: "no", in: "n\n", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &UI{noColor: true, reader: bufio.NewReader(strings.NewReader(tc.in))}
			var got bool
			captureStdout(t, func() {
				got = u.Confirm("ok?")
			})
			if got != tc.want {
				t.Fatalf("Confirm() = %v, want %v", got, tc.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })
	fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	_ = r.Close()
	return buf.String()
}
