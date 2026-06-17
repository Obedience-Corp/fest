package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/tui/picker"
)

func writeProgressFestival(t *testing.T, total, completed int) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("name: test\nid: TEST-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seq := filepath.Join(dir, "001_PHASE", "01_sequence")
	if err := os.MkdirAll(seq, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < total; i++ {
		content := fmt.Sprintf("---\nfest_type: task\nfest_status: pending\n---\n# Task %d\n", i+1)
		if err := os.WriteFile(filepath.Join(seq, fmt.Sprintf("%02d_task.md", i+1)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := progress.NewManager(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < completed; i++ {
		if err := mgr.MarkComplete(t.Context(), fmt.Sprintf("001_PHASE/01_sequence/%02d_task.md", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFestivalProgressDetail(t *testing.T) {
	withTasks := writeProgressFestival(t, 4, 2)
	detail := festivalProgressDetail(t.Context(), withTasks)
	if detail == "" {
		t.Fatal("expected a progress bar for a festival with tasks")
	}
	if !strings.Contains(detail, "50%") {
		t.Fatalf("expected 50%% in progress detail, got %q", detail)
	}

	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "fest.yaml"), []byte("name: empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := festivalProgressDetail(t.Context(), empty); d != "" {
		t.Fatalf("expected no bar for a festival with no tasks, got %q", d)
	}
}

func TestAttachProgressDetails(t *testing.T) {
	withTasks := writeProgressFestival(t, 2, 1)
	items := []picker.Item{
		{Name: "[active] has-tasks", Value: withTasks},
		{Name: "[active]/", Value: ""},
	}
	attachProgressDetails(t.Context(), items)
	if !strings.Contains(items[0].Detail, "%") {
		t.Fatalf("expected a progress bar for the festival item, got %q", items[0].Detail)
	}
	if items[1].Detail != "" {
		t.Fatalf("expected no detail for an item without a path, got %q", items[1].Detail)
	}
}

func TestFestivalPickCandidatesForGoIncludeStatusDirectories(t *testing.T) {
	candidates := filterFestivalPickCandidates(pureCandidateFixture(), FestivalPickerOptions{
		IncludeStatusDirectories: true,
	})

	if !hasCandidate(candidates, "active", true) {
		t.Fatal("fest go picker candidates should include active status directory")
	}
	if !hasCandidate(candidates, "planning", true) {
		t.Fatal("fest go picker candidates should include planning status directory")
	}
}

func TestFestivalPickCandidatesForWatchExcludeStatusDirectories(t *testing.T) {
	candidates := filterFestivalPickCandidates(pureCandidateFixture(), FestivalPickerOptions{
		IncludeStatusDirectories: false,
	})

	for _, candidate := range candidates {
		if candidate.StatusDirectory {
			t.Fatalf("fest watch picker candidates should exclude status directory %q", candidate.Name)
		}
	}
}

func TestFestivalPickCandidatesPreferredStatusFiltersCandidates(t *testing.T) {
	candidates := filterFestivalPickCandidates(pureCandidateFixture(), FestivalPickerOptions{
		PreferredStatuses: []string{"active"},
	})

	if len(candidates) != 1 {
		t.Fatalf("expected 1 active festival candidate, got %d: %#v", len(candidates), candidates)
	}
	if candidates[0].Status != "active" || candidates[0].Name != "active-work-AW0001" {
		t.Fatalf("unexpected preferred-status candidate: %#v", candidates[0])
	}
}

func TestFestivalPickerItemsPreserveStatusPathAndLabel(t *testing.T) {
	items := festivalPickerItemsFromCandidates([]FestivalPickCandidate{
		{
			Name:   "active-work-AW0001",
			Path:   "/campaign/festivals/active/active-work-AW0001",
			Status: "active",
		},
		{
			Name:            "active",
			Path:            "/campaign/festivals/active",
			Status:          "active",
			StatusDirectory: true,
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 picker items, got %d", len(items))
	}
	if items[0].Name != "[active] active-work-AW0001" {
		t.Fatalf("festival item label = %q", items[0].Name)
	}
	if items[0].Value != "/campaign/festivals/active/active-work-AW0001" {
		t.Fatalf("festival item value = %q", items[0].Value)
	}
	if items[1].Name != "[active]/" {
		t.Fatalf("status item label = %q", items[1].Name)
	}
	if items[1].Value != "/campaign/festivals/active" {
		t.Fatalf("status item value = %q", items[1].Value)
	}
}

func TestFestivalPickCandidateRequiresFestivalMarker(t *testing.T) {
	if isFestivalPickCandidate("id-shaped-only-IS0001", "/path/that/does/not/exist", FestivalPickerOptions{}) {
		t.Fatal("ID-shaped directory names must not be treated as watchable festivals without festival markers")
	}
}

func TestFestivalPickCandidateCanIncludeUnmarkedForNavigation(t *testing.T) {
	if !isFestivalPickCandidate("id-shaped-only-IS0001", "/path/that/does/not/exist", FestivalPickerOptions{
		IncludeUnmarkedFestivalDirectories: true,
	}) {
		t.Fatal("fest go picker compatibility should allow unmarked status child directories")
	}
}

func TestDefaultCandidateStatusesPrioritizeActiveForPicker(t *testing.T) {
	statuses := defaultCandidateStatuses()

	active := indexOf(statuses, "active")
	planning := indexOf(statuses, "planning")
	if active < 0 || planning < 0 {
		t.Fatalf("default statuses missing active or planning: %v", statuses)
	}
	if active > planning {
		t.Fatalf("active should be ordered before planning for picker defaults: %v", statuses)
	}
}

func pureCandidateFixture() []FestivalPickCandidate {
	return []FestivalPickCandidate{
		{
			Name:            "active",
			Path:            "/campaign/festivals/active",
			Status:          "active",
			StatusDirectory: true,
		},
		{
			Name:   "active-work-AW0001",
			ID:     "AW0001",
			Path:   "/campaign/festivals/active/active-work-AW0001",
			Status: "active",
		},
		{
			Name:            "planning",
			Path:            "/campaign/festivals/planning",
			Status:          "planning",
			StatusDirectory: true,
		},
		{
			Name:   "planning-work-PW0001",
			ID:     "PW0001",
			Path:   "/campaign/festivals/planning/planning-work-PW0001",
			Status: "planning",
		},
	}
}

func hasCandidate(candidates []FestivalPickCandidate, name string, statusDirectory bool) bool {
	for _, candidate := range candidates {
		if candidate.Name == name && candidate.StatusDirectory == statusDirectory {
			return true
		}
	}
	return false
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}
