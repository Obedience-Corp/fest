package tokens

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupDigits(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{15787315, "15,787,315"},
		{-4200, "-4,200"},
	}
	for _, tt := range tests {
		if got := groupDigits(tt.in); got != tt.want {
			t.Errorf("groupDigits(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveTarget_PathAndAllConflict(t *testing.T) {
	if _, err := resolveTarget("some/path", true); err == nil {
		t.Fatal("expected validation error for path with --all")
	}
}

func TestResolveTarget_ExplicitPathPassesThrough(t *testing.T) {
	got, err := resolveTarget("docs/plan", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "docs/plan" {
		t.Fatalf("resolveTarget = %q", got)
	}
}

func TestResolveTarget_OutsideFestivalErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := resolveTarget("", false)
	if err == nil {
		t.Fatal("expected not-found error outside a festival")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Fatalf("hint should offer --all: %v", err)
	}
}

func TestRunTokens_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runTokens(ctx, t.TempDir(), &tokensOptions{}); err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestRunTokens_MissingPath(t *testing.T) {
	err := runTokens(context.Background(), filepath.Join(t.TempDir(), "nope"), &tokensOptions{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fnErr := fn()
	os.Stdout = old
	_ = w.Close()
	out, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("reading captured stdout: %v", readErr)
	}
	return string(out), fnErr
}

func TestRunTokens_JSONOnFixtureDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PLAN.md"), []byte(strings.Repeat("festival planning corpus ", 50)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "GOAL.md"), []byte("ship the launch"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return runTokens(context.Background(), dir, &tokensOptions{json: true})
	})
	if err != nil {
		t.Fatalf("runTokens: %v", err)
	}
	var payload struct {
		Path        string `json:"path"`
		Tokens      int    `json:"tokens"`
		Method      string `json:"method"`
		DisplayName string `json:"display_name"`
		FileCount   int    `json:"file_count"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON output %q: %v", out, err)
	}
	if payload.Tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", payload.Tokens)
	}
	if payload.FileCount != 2 {
		t.Fatalf("file_count = %d, want 2", payload.FileCount)
	}
	if payload.Method == "" || payload.Path != dir {
		t.Fatalf("payload missing fields: %+v", payload)
	}
}

func TestRunTokens_HumanOutputGroupsDigits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "BIG.md"), []byte(strings.Repeat("magnitude evidence for the launch story ", 800)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return runTokens(context.Background(), dir, &tokensOptions{})
	})
	if err != nil {
		t.Fatalf("runTokens: %v", err)
	}
	if !strings.Contains(out, "tokens") || !strings.Contains(out, dir) {
		t.Fatalf("human output missing tokens/path: %q", out)
	}
	if !strings.Contains(out, ",") {
		t.Fatalf("expected grouped digits in %q", out)
	}
}
