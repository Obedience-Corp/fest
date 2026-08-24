package ritual

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindFestivalForConvert(t *testing.T) {
	root := t.TempDir()
	alpha := mkdirConvertFest(t, root, "planning", "alpha-security-AS0001")
	_ = mkdirConvertFest(t, root, "planning", "beta-security-BS0001")
	unique := mkdirConvertFest(t, root, "ready", "unique-review-UR0001")
	dungeon := mkdirConvertFest(t, root, filepath.Join("dungeon", "completed", "2026-01"), "done-job-DJ0001")
	active := mkdirConvertFest(t, root, "active", "exact-name-EN0001")

	ctx := context.Background()

	t.Run("exact directory name", func(t *testing.T) {
		got, status, err := findFestivalForConvert(ctx, root, "exact-name-EN0001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != active || status != "active" {
			t.Fatalf("got path=%q status=%q, want path=%q status=active", got, status, active)
		}
	})

	t.Run("exact ID", func(t *testing.T) {
		got, status, err := findFestivalForConvert(ctx, root, "UR0001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != unique || status != "ready" {
			t.Fatalf("got path=%q status=%q, want path=%q status=ready", got, status, unique)
		}
	})

	t.Run("unique substring", func(t *testing.T) {
		got, status, err := findFestivalForConvert(ctx, root, "unique-review")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != unique || status != "ready" {
			t.Fatalf("got path=%q status=%q, want unique-review", got, status)
		}
	})

	t.Run("ambiguous substring", func(t *testing.T) {
		_, _, err := findFestivalForConvert(ctx, root, "security")
		if err == nil {
			t.Fatal("expected ambiguous error")
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "ambiguous") {
			t.Fatalf("error = %v, want ambiguous", err)
		}
	})

	t.Run("exact ID among substring-colliding names", func(t *testing.T) {
		got, status, err := findFestivalForConvert(ctx, root, "AS0001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != alpha || status != "planning" {
			t.Fatalf("got path=%q status=%q, want alpha-security", got, status)
		}
	})

	t.Run("dungeon date bucket", func(t *testing.T) {
		got, status, err := findFestivalForConvert(ctx, root, "DJ0001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dungeon || status != "dungeon/completed" {
			t.Fatalf("got path=%q status=%q, want dungeon completed", got, status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, _, err := findFestivalForConvert(ctx, root, "does-not-exist")
		if err == nil {
			t.Fatal("expected not found")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Fatalf("error = %v, want not found", err)
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := findFestivalForConvert(cancelled, root, "UR0001")
		if err == nil {
			t.Fatal("expected context cancellation error")
		}
	})
}

func mkdirConvertFest(t *testing.T, root, status, name string) string {
	t.Helper()
	path := filepath.Join(root, status, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return path
}
