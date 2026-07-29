package list

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"golang.org/x/term"
)

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"planning", true},
		{"dungeon/completed", true},
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
		{"created", true},
		{"updated", true},
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

func TestApplySorting_ByCreated(t *testing.T) {
	now := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "oldest", CreatedAt: now.Add(-3 * time.Hour)},
		{Name: "newest", CreatedAt: now.Add(-1 * time.Hour)},
		{Name: "middle", CreatedAt: now.Add(-2 * time.Hour)},
	}

	applySorting(festivals, "created", false)

	expected := []string{"newest", "middle", "oldest"}
	for i, want := range expected {
		if festivals[i].Name != want {
			t.Errorf("position %d: got %s, want %s", i, festivals[i].Name, want)
		}
	}
}

func TestApplySorting_ByUpdated(t *testing.T) {
	now := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "stale", UpdatedAt: now.Add(-3 * time.Hour)},
		{Name: "fresh", UpdatedAt: now.Add(-1 * time.Hour)},
		{Name: "medium", UpdatedAt: now.Add(-2 * time.Hour)},
	}

	applySorting(festivals, "updated", false)

	expected := []string{"fresh", "medium", "stale"}
	for i, want := range expected {
		if festivals[i].Name != want {
			t.Errorf("position %d: got %s, want %s", i, festivals[i].Name, want)
		}
	}
}

func TestParseFilterDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"YYYY-MM-DD", "2026-01-15", false},
		{"RFC3339", "2026-01-15T10:30:00Z", false},
		{"RFC3339 with offset", "2026-01-15T10:30:00-07:00", false},
		{"invalid format", "01/15/2026", true},
		{"garbage", "not-a-date", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFilterDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFilterDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseFilterDate_Values(t *testing.T) {
	t.Run("YYYY-MM-DD returns midnight", func(t *testing.T) {
		got, err := parseFilterDate("2026-03-15")
		if err != nil {
			t.Fatal(err)
		}
		if got.Year() != 2026 || got.Month() != 3 || got.Day() != 15 {
			t.Errorf("got %v, want 2026-03-15", got)
		}
	})

	t.Run("RFC3339 preserves time", func(t *testing.T) {
		got, err := parseFilterDate("2026-03-15T14:30:00Z")
		if err != nil {
			t.Fatal(err)
		}
		if got.Hour() != 14 || got.Minute() != 30 {
			t.Errorf("got %v, want 14:30", got)
		}
	})
}

func TestApplyFilters(t *testing.T) {
	now := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "fest-a", ProjectPath: "/path/to/camp", CreatedAt: now.Add(-48 * time.Hour)},
		{Name: "fest-b", ProjectPath: "/path/to/fest", CreatedAt: now.Add(-24 * time.Hour)},
		{Name: "fest-c", ProjectPath: "/path/to/camp/sub", CreatedAt: now},
	}

	tests := []struct {
		name          string
		filterProject string
		since         string
		until         string
		wantCount     int
		wantNames     []string
		wantErr       bool
	}{
		{name: "no filters", wantCount: 3},
		{name: "project filter match", filterProject: "camp", wantCount: 2, wantNames: []string{"fest-a", "fest-c"}},
		{name: "project filter no match", filterProject: "daemon", wantCount: 0},
		{name: "project filter case insensitive", filterProject: "CAMP", wantCount: 2},
		{name: "since filter", since: now.Add(-25 * time.Hour).Format(time.RFC3339), wantCount: 2},
		{name: "until filter", until: now.Add(-49 * time.Hour).Format(time.RFC3339), wantCount: 1, wantNames: []string{"fest-a"}},
		{name: "combined filters", filterProject: "camp", since: now.Add(-1 * time.Hour).Format(time.RFC3339), wantCount: 1, wantNames: []string{"fest-c"}},
		{name: "invalid since date", since: "not-a-date", wantErr: true},
		{name: "invalid until date", until: "13/13/2026", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &listOptions{
				filterProject: tt.filterProject,
				since:         tt.since,
				until:         tt.until,
			}
			result, err := applyFilters(festivals, opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				names := make([]string, len(result))
				for i, f := range result {
					names[i] = f.Name
				}
				t.Errorf("got %d results %v, want %d", len(result), names, tt.wantCount)
			}
			if tt.wantNames != nil {
				for i, name := range tt.wantNames {
					if i < len(result) && result[i].Name != name {
						t.Errorf("position %d: got %s, want %s", i, result[i].Name, name)
					}
				}
			}
		})
	}
}

func TestApplyFilters_EmptyList(t *testing.T) {
	result, err := applyFilters([]*show.FestivalInfo{}, &listOptions{filterProject: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestApplyFilters_ZeroTimestamps(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "no-date", CreatedAt: time.Time{}},
		{Name: "has-date", CreatedAt: time.Now()},
	}

	result, err := applyFilters(festivals, &listOptions{since: "2020-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	// Festivals with zero CreatedAt pass through date filters
	// because there's no date to compare against
	if len(result) != 2 {
		names := make([]string, len(result))
		for i, f := range result {
			names[i] = f.Name
		}
		t.Errorf("expected 2 results (zero timestamps pass through), got %d: %v", len(result), names)
	}
}

func TestFestivalsToMap_IncludesTimestamps(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	festivals := []*show.FestivalInfo{
		{Name: "test-fest", Path: "/tmp/test", Status: "active",
			CreatedAt: now, UpdatedAt: now.Add(time.Hour)},
	}
	result := festivalsToMap(festivals)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if _, ok := result[0]["created_at"]; !ok {
		t.Error("missing created_at in JSON map")
	}
	if _, ok := result[0]["updated_at"]; !ok {
		t.Error("missing updated_at in JSON map")
	}
}

func TestFestivalsToMap_OmitsZeroTimestamps(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "test-fest", Path: "/tmp/test", Status: "active"},
	}
	result := festivalsToMap(festivals)
	if _, ok := result[0]["created_at"]; ok {
		t.Error("zero-value created_at should be omitted from JSON map")
	}
	if _, ok := result[0]["updated_at"]; ok {
		t.Error("zero-value updated_at should be omitted from JSON map")
	}
}

func TestFestivalsToMapWithProgress_IncludesTimestamps(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	festivals := []*show.FestivalInfo{
		{Name: "test-fest", Path: "/tmp/test", Status: "active",
			CreatedAt: now, UpdatedAt: now.Add(time.Hour)},
	}
	result := festivalsToMapWithProgress(festivals, nil)
	if _, ok := result[0]["created_at"]; !ok {
		t.Error("missing created_at in progress JSON map")
	}
	if _, ok := result[0]["updated_at"]; !ok {
		t.Error("missing updated_at in progress JSON map")
	}
}

func TestFestivalsToMap_IncludesStatusDate(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{
			Name:       "dungeoned-fest",
			Path:       "/tmp/dungeon/completed/2026-02-10/dungeoned-fest",
			Status:     "dungeon/completed",
			StatusDate: "2026-02-10",
		},
	}
	result := festivalsToMap(festivals)
	got, ok := result[0]["status_date"]
	if !ok {
		t.Fatal("missing status_date in JSON map")
	}
	if got != "2026-02-10" {
		t.Errorf("status_date = %v, want %q", got, "2026-02-10")
	}
}

func TestFestivalsToMap_OmitsEmptyStatusDate(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "active-fest", Path: "/tmp/active/fest", Status: "active"},
	}
	result := festivalsToMap(festivals)
	if _, ok := result[0]["status_date"]; ok {
		t.Error("empty StatusDate should be omitted from JSON map")
	}
}

func TestFestivalsToMapWithProgress_IncludesStatusDate(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{
			Name:       "dungeoned-fest",
			Path:       "/tmp/dungeon/completed/2026-02-10/dungeoned-fest",
			Status:     "dungeon/completed",
			StatusDate: "2026-02-10",
		},
	}
	result := festivalsToMapWithProgress(festivals, nil)
	got, ok := result[0]["status_date"]
	if !ok {
		t.Fatal("missing status_date in progress JSON map")
	}
	if got != "2026-02-10" {
		t.Errorf("status_date = %v, want %q", got, "2026-02-10")
	}
}

func TestSortByStatusDate_NewestBucketFirst(t *testing.T) {
	now := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "older", StatusDate: "2026-01-19", ModTime: now},
		{Name: "newest", StatusDate: "2026-02-10", ModTime: now.Add(-time.Hour)},
		{Name: "middle", StatusDate: "2026-01-26", ModTime: now},
		{Name: "no-bucket", StatusDate: "", ModTime: now},
	}
	sortByStatusDate(festivals)
	want := []string{"newest", "middle", "older", "no-bucket"}
	for i, name := range want {
		if festivals[i].Name != name {
			t.Errorf("position %d: got %q, want %q", i, festivals[i].Name, name)
		}
	}
}

func TestSortByStatusDate_EqualBucketsFallBackToModTime(t *testing.T) {
	base := time.Now()
	festivals := []*show.FestivalInfo{
		{Name: "older-mod", StatusDate: "2026-02-10", ModTime: base.Add(-time.Hour)},
		{Name: "newer-mod", StatusDate: "2026-02-10", ModTime: base},
	}
	sortByStatusDate(festivals)
	if festivals[0].Name != "newer-mod" {
		t.Errorf("tie-break by ModTime: position 0 = %q, want %q", festivals[0].Name, "newer-mod")
	}
}

func TestSortByStatusDate_EmptyStatusDateSinksToEnd(t *testing.T) {
	festivals := []*show.FestivalInfo{
		{Name: "empty-first", StatusDate: ""},
		{Name: "dated", StatusDate: "2026-01-01"},
		{Name: "empty-last", StatusDate: ""},
	}
	sortByStatusDate(festivals)
	if festivals[0].Name != "dated" {
		t.Errorf("position 0: got %q, want %q", festivals[0].Name, "dated")
	}
	if festivals[1].StatusDate != "" || festivals[2].StatusDate != "" {
		t.Errorf("empty StatusDate entries should be at positions 1-2, got %q, %q",
			festivals[1].StatusDate, festivals[2].StatusDate)
	}
}

func TestListCommandExposesWatchFlag(t *testing.T) {
	cmd := NewListCommand()
	if cmd.Flags().Lookup("watch") == nil {
		t.Fatal("expected --watch flag on fest list")
	}
	if cmd.Flags().ShorthandLookup("w") == nil {
		t.Fatal("expected -w shorthand for --watch")
	}
}

func TestListWatchRejectsJSON(t *testing.T) {
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--watch", "--json"})
	// Avoid terminal check failing first if order matters; both flags set.
	listStdoutIsTerminal = func() bool { return true }
	t.Cleanup(func() {
		listStdoutIsTerminal = func() bool {
			return term.IsTerminal(int(os.Stdout.Fd()))
		}
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --watch --json")
	}
	if !strings.Contains(err.Error(), "--watch cannot be combined with --json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListWatchRejectsNonTTY(t *testing.T) {
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--watch"})
	listStdoutIsTerminal = func() bool { return false }
	t.Cleanup(func() {
		listStdoutIsTerminal = func() bool {
			return term.IsTerminal(int(os.Stdout.Fd()))
		}
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --watch on non-TTY")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("unexpected error: %v", err)
	}
}
