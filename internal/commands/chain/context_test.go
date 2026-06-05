package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFestivalConfig(t *testing.T, dir, festivalID string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: \"1.0\"\nmetadata:\n  id: " + festivalID +
		"\n  status_history:\n    - status: active\n"
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFirstArg(t *testing.T) {
	if got := firstArg(nil); got != "" {
		t.Fatalf("firstArg(nil) = %q, want empty", got)
	}
	if got := firstArg([]string{"CH0001"}); got != "CH0001" {
		t.Fatalf("firstArg = %q, want CH0001", got)
	}
}

func TestFestivalIDFromMarkers(t *testing.T) {
	root := t.TempDir()
	fest := filepath.Join(root, "festivals", "active", "demo-FA0010")
	writeFestivalConfig(t, fest, "FA0010")

	if got := festivalIDFromMarkers(fest); got != "FA0010" {
		t.Fatalf("from festival dir = %q, want FA0010", got)
	}

	nested := filepath.Join(fest, "001_PHASE", "01_seq")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := festivalIDFromMarkers(nested); got != "FA0010" {
		t.Fatalf("from nested dir = %q, want FA0010 (should walk up)", got)
	}

	if got := festivalIDFromMarkers(root); got != "" {
		t.Fatalf("from non-festival dir = %q, want empty", got)
	}
}

func TestResolveChainID_ExplicitWins(t *testing.T) {
	chainID, inferred, err := resolveChainID(context.Background(), "CH0001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chainID != "CH0001" {
		t.Fatalf("chainID = %q, want CH0001", chainID)
	}
	if inferred {
		t.Fatal("explicit id must not be marked inferred")
	}
}

func TestResolveCurrentFestivalID_FromMarkers(t *testing.T) {
	root := t.TempDir()
	fest := filepath.Join(root, "festivals", "active", "demo-FA0010")
	writeFestivalConfig(t, fest, "FA0010")
	t.Chdir(fest)

	got, ok := resolveCurrentFestivalID(context.Background())
	if !ok || got != "FA0010" {
		t.Fatalf("resolveCurrentFestivalID = %q, %v; want FA0010, true", got, ok)
	}
}

func TestResolveCurrentFestivalID_NoContext(t *testing.T) {
	t.Chdir(t.TempDir())
	if got, ok := resolveCurrentFestivalID(context.Background()); ok {
		t.Fatalf("expected no festival context, got %q", got)
	}
}

func TestPickChainID_NonInteractiveReturnsEmpty(t *testing.T) {
	// Tests run without a TTY, so the picker must no-op rather than block.
	got, err := pickChainID(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("non-interactive pickChainID = %q, want empty", got)
	}
}

func TestResolveChainID_NoContextNonInteractiveErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "festivals", ".festival"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "festivals"))

	_, _, err := resolveChainID(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when no chain id, no inferable context, and not a TTY")
	}
}

func TestConfirmChainCompletion_NonInteractiveRefuses(t *testing.T) {
	if confirmChainCompletion("CH0001") {
		t.Fatal("non-interactive confirmation must refuse (return false)")
	}
}
