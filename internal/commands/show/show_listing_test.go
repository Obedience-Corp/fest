package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListFestivalsByStatus(t *testing.T) {
	tmpDir := t.TempDir()

	// Create status directories with festivals
	activeDir := filepath.Join(tmpDir, "active")
	planningDir := filepath.Join(tmpDir, "planning")

	festival1 := filepath.Join(activeDir, "fest1")
	festival2 := filepath.Join(activeDir, "fest2")
	festival3 := filepath.Join(planningDir, "fest3")

	for _, d := range []string{festival1, festival2, festival3} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		// Add festival goal file to make it valid
		if err := os.WriteFile(filepath.Join(d, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test active festivals
	active, err := ListFestivalsByStatus(context.Background(), tmpDir, "active", "")
	if err != nil {
		t.Fatalf("ListFestivalsByStatus(active) unexpected error: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("ListFestivalsByStatus(active) returned %d festivals, want 2", len(active))
	}

	// Test planning festivals
	planning, err := ListFestivalsByStatus(context.Background(), tmpDir, "planning", "")
	if err != nil {
		t.Fatalf("ListFestivalsByStatus(planning) unexpected error: %v", err)
	}
	if len(planning) != 1 {
		t.Errorf("ListFestivalsByStatus(planning) returned %d festivals, want 1", len(planning))
	}

	// Test non-existent status
	empty, err := ListFestivalsByStatus(context.Background(), tmpDir, "completed", "")
	if err != nil {
		t.Fatalf("ListFestivalsByStatus(completed) unexpected error: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ListFestivalsByStatus(completed) returned %d festivals, want 0", len(empty))
	}
}

// TestListFestivalsByStatus_DateDirectories tests that listing festivals finds festivals
// inside date subdirectories.
func TestListFestivalsByStatus_DateDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create festivals in date subdirectories
	dateDirFest := filepath.Join(tmpDir, "dungeon", "completed", "2026-02-28", "fest-in-date-dir")
	directFest := filepath.Join(tmpDir, "dungeon", "completed", "fest-direct")

	for _, d := range []string{dateDirFest, directFest} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	festivals, err := ListFestivalsByStatus(context.Background(), tmpDir, "dungeon/completed", "")
	if err != nil {
		t.Fatalf("ListFestivalsByStatus() error = %v", err)
	}

	if len(festivals) != 2 {
		t.Errorf("expected 2 festivals (direct + date dir), got %d", len(festivals))
		for _, f := range festivals {
			t.Logf("  found: %s at %s", f.Name, f.Path)
		}
	}

	// Verify both are found
	names := map[string]bool{}
	for _, f := range festivals {
		names[f.Name] = true
	}
	if !names["fest-in-date-dir"] {
		t.Error("festival in date directory not found")
	}
	if !names["fest-direct"] {
		t.Error("direct festival not found")
	}
}

// TestFindFestivalByName_DateDirectories tests that searching finds festivals inside date dirs.
func TestFindFestivalByName_DateDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up full status directory structure
	for _, status := range []string{"planning", "ready", "active", "dungeon/completed", "dungeon/archived", "dungeon/someday"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, status), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a festival inside a date directory
	festDir := filepath.Join(tmpDir, "dungeon", "completed", "2026-02-28", "my-completed-fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := FindFestivalByName(context.Background(), tmpDir, "my-completed-fest", "")
	if err != nil {
		t.Fatalf("FindFestivalByName() error = %v", err)
	}

	if info.Name != "my-completed-fest" {
		t.Errorf("Name = %q, want %q", info.Name, "my-completed-fest")
	}
	if info.Status != "dungeon/completed" {
		t.Errorf("Status = %q, want %q", info.Status, "dungeon/completed")
	}
	if info.StatusDate != "2026-02-28" {
		t.Errorf("StatusDate = %q, want %q", info.StatusDate, "2026-02-28")
	}
}

// TestListFestivalsByStatus_PopulatesStatusDate verifies that ListFestivalsByStatus
// records the dated bucket name on each FestivalInfo returned from a dungeon
// status, and leaves StatusDate empty for non-bucketed entries.
func TestListFestivalsByStatus_PopulatesStatusDate(t *testing.T) {
	tmpDir := t.TempDir()

	dateDirFest := filepath.Join(tmpDir, "dungeon", "completed", "2026-02-28", "fest-in-date-dir")
	directFest := filepath.Join(tmpDir, "dungeon", "completed", "fest-direct")

	for _, d := range []string{dateDirFest, directFest} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	festivals, err := ListFestivalsByStatus(context.Background(), tmpDir, "dungeon/completed", "")
	if err != nil {
		t.Fatalf("ListFestivalsByStatus() error = %v", err)
	}

	byName := map[string]*FestivalInfo{}
	for _, f := range festivals {
		byName[f.Name] = f
	}

	bucketed, ok := byName["fest-in-date-dir"]
	if !ok {
		t.Fatalf("bucketed festival not found; got %v", byName)
	}
	if bucketed.StatusDate != "2026-02-28" {
		t.Errorf("StatusDate for bucketed festival = %q, want %q", bucketed.StatusDate, "2026-02-28")
	}

	direct, ok := byName["fest-direct"]
	if !ok {
		t.Fatalf("direct festival not found; got %v", byName)
	}
	if direct.StatusDate != "" {
		t.Errorf("StatusDate for non-bucketed festival = %q, want empty", direct.StatusDate)
	}
}

// TestListFestivalsByStatusLight_PopulatesStatusDate is the same assertion
// against the light (no-stats) discovery path.
func TestListFestivalsByStatusLight_PopulatesStatusDate(t *testing.T) {
	tmpDir := t.TempDir()

	festDir := filepath.Join(tmpDir, "dungeon", "someday", "2026-04-01", "maybe-later-ML0001")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	festivals, err := ListFestivalsByStatusLight(context.Background(), tmpDir, "dungeon/someday")
	if err != nil {
		t.Fatalf("ListFestivalsByStatusLight() error = %v", err)
	}
	if len(festivals) != 1 {
		t.Fatalf("expected 1 festival, got %d", len(festivals))
	}
	if got := festivals[0].StatusDate; got != "2026-04-01" {
		t.Errorf("StatusDate = %q, want %q", got, "2026-04-01")
	}
}

// TestParseFestivalInfo_StatusDateFromBucket verifies that the single-festival
// parse path records StatusDate when walked directly into a dated bucket.
func TestParseFestivalInfo_StatusDateFromBucket(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, "dungeon", "completed", "2026-02-28", "lone-fest-LF0001")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := parseFestivalInfo(context.Background(), festDir, "")
	if err != nil {
		t.Fatalf("parseFestivalInfo() error = %v", err)
	}
	if info.Status != "dungeon/completed" {
		t.Errorf("Status = %q, want %q", info.Status, "dungeon/completed")
	}
	if info.StatusDate != "2026-02-28" {
		t.Errorf("StatusDate = %q, want %q", info.StatusDate, "2026-02-28")
	}
}

// TestParseFestivalInfo_NoStatusDateForActive verifies that non-dungeon
// festivals do not get a StatusDate.
func TestParseFestivalInfo_NoStatusDateForActive(t *testing.T) {
	tmpDir := t.TempDir()
	festDir := filepath.Join(tmpDir, "active", "active-fest-AF0001")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, FestivalGoalFile), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := parseFestivalInfo(context.Background(), festDir, "")
	if err != nil {
		t.Fatalf("parseFestivalInfo() error = %v", err)
	}
	if info.Status != "active" {
		t.Errorf("Status = %q, want %q", info.Status, "active")
	}
	if info.StatusDate != "" {
		t.Errorf("StatusDate = %q, want empty", info.StatusDate)
	}
}
