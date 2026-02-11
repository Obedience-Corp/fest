package list

import (
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"planning", true},
		{"completed", true},
		{"dungeon", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := isValidStatus(tt.status); got != tt.want {
				t.Errorf("isValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsValidSortBy(t *testing.T) {
	tests := []struct {
		sort string
		want bool
	}{
		{"date", true},
		{"status", true},
		{"progress", true},
		{"name", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			if got := isValidSortBy(tt.sort); got != tt.want {
				t.Errorf("isValidSortBy(%q) = %v, want %v", tt.sort, got, tt.want)
			}
		})
	}
}

func TestStatusOrder(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{"active", 0},
		{"ready", 1},
		{"planning", 2},
		{"completed", 3},
		{"dungeon", 4},
		{"dungeon/completed", 5},
		{"dungeon/archived", 6},
		{"dungeon/someday", 7},
		{"unknown", 8},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusOrder(tt.status); got != tt.want {
				t.Errorf("statusOrder(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

func TestApplySorting_ByName(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}

	applySorting(festivals, "name", false)

	if festivals[0].Name != "alpha" || festivals[1].Name != "bravo" || festivals[2].Name != "charlie" {
		t.Errorf("expected alpha, bravo, charlie; got %s, %s, %s",
			festivals[0].Name, festivals[1].Name, festivals[2].Name)
	}
}

func TestApplySorting_ByStatus(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "c", Status: "completed"},
		{Name: "a", Status: "active"},
		{Name: "p", Status: "planning"},
	}

	applySorting(festivals, "status", false)

	if festivals[0].Status != "active" || festivals[1].Status != "planning" || festivals[2].Status != "completed" {
		t.Errorf("expected active, planning, completed; got %s, %s, %s",
			festivals[0].Status, festivals[1].Status, festivals[2].Status)
	}
}

func TestApplySorting_ByProgress(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "low", Stats: &show.FestivalStats{Progress: 10.0}},
		{Name: "high", Stats: &show.FestivalStats{Progress: 90.0}},
		{Name: "mid", Stats: &show.FestivalStats{Progress: 50.0}},
	}

	applySorting(festivals, "progress", false)

	if festivals[0].Name != "high" || festivals[1].Name != "mid" || festivals[2].Name != "low" {
		t.Errorf("expected high, mid, low; got %s, %s, %s",
			festivals[0].Name, festivals[1].Name, festivals[2].Name)
	}
}

func TestApplySorting_ByDate(t *testing.T) {
	now := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "old", ModTime: now.Add(-24 * time.Hour)},
		{Name: "new", ModTime: now},
		{Name: "mid", ModTime: now.Add(-12 * time.Hour)},
	}

	applySorting(festivals, "date", false)

	if festivals[0].Name != "new" || festivals[1].Name != "mid" || festivals[2].Name != "old" {
		t.Errorf("expected new, mid, old; got %s, %s, %s",
			festivals[0].Name, festivals[1].Name, festivals[2].Name)
	}
}

func TestApplySorting_DefaultWithAlpha(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}

	applySorting(festivals, "", true)

	if festivals[0].Name != "alpha" {
		t.Errorf("expected alpha first with alpha=true, got %s", festivals[0].Name)
	}
}

func TestApplySorting_ProgressNilStats(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "no-stats"},
		{Name: "has-stats", Stats: &show.FestivalStats{Progress: 50.0}},
	}

	applySorting(festivals, "progress", false)

	if festivals[0].Name != "has-stats" {
		t.Errorf("expected has-stats first (50%% > 0%%); got %s", festivals[0].Name)
	}
}
