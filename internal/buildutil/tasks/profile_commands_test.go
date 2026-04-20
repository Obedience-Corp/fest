package tasks

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestParseCommandSurfaceOutput(t *testing.T) {
	output := []byte("\nfest alpha\n\n  fest beta  \nfest gamma\n")

	got := parseCommandSurfaceOutput(output)
	want := []string{"fest alpha", "fest beta", "fest gamma"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCommandSurfaceOutput() = %#v, want %#v", got, want)
	}
}

func TestDiffCommandSurfaces(t *testing.T) {
	base := []string{"fest alpha", "fest beta"}
	candidate := []string{"fest alpha", "fest beta", "fest explore", "fest explore active"}

	got := diffCommandSurfaces(base, candidate)
	want := []string{"fest explore", "fest explore active"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffCommandSurfaces() = %#v, want %#v", got, want)
	}
}

func TestPrintCommandProfile(t *testing.T) {
	output := captureStdout(t, func() {
		printCommandProfile("stable", []string{"fest alpha", "fest beta"})
	})

	want := "== stable profile (2 commands) ==\nfest alpha\nfest beta\n"
	if output != want {
		t.Fatalf("printCommandProfile() = %q, want %q", output, want)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy(): %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}
