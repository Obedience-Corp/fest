package show

import (
	"regexp"
	"strings"
	"testing"
)

func TestFormatFestivalDetails(t *testing.T) {
	festival := &FestivalInfo{
		ID:     "test-fest",
		Name:   "test-fest",
		Status: "active",
		Path:   "/path/to/test-fest",
		Stats: &FestivalStats{
			Phases: StatusCounts{Total: 3, Completed: 1, InProgress: 1, Pending: 1},
			Tasks:  StatusCounts{Total: 10, Completed: 5, InProgress: 2, Pending: 3},
		},
	}
	festival.Stats.Progress = 50.0

	output := FormatFestivalDetails(festival, false, "")

	// Check that key elements are present
	if !contains(output, "test-fest") {
		t.Error("Output should contain festival name")
	}
	if !contains(output, "active") {
		t.Error("Output should contain status")
	}
	if !contains(output, "50.0%") {
		t.Error("Output should contain progress percentage")
	}
}

func TestFormatFestivalList(t *testing.T) {
	festivals := []*FestivalInfo{
		{Name: "fest1", Stats: &FestivalStats{Progress: 25}},
		{Name: "fest2", Stats: &FestivalStats{Progress: 75}},
	}

	output := FormatFestivalList("active", festivals, nil)

	if !contains(output, "Festivals (2)") {
		t.Error("Output should contain header with count")
	}
	if !contains(output, "fest1") {
		t.Error("Output should contain festival names")
	}
	if !contains(output, "[25%]") {
		t.Error("Output should contain progress")
	}
}

func TestFormatFestivalListEmpty(t *testing.T) {
	output := FormatFestivalList("completed", []*FestivalInfo{}, nil)

	if !contains(output, "Festivals (0)") {
		t.Error("Output should indicate zero festivals")
	}
	if !contains(output, "(none)") {
		t.Error("Output should indicate no festivals")
	}
}

// TestFormatFestivalDetails_DisplaysMetadataID tests that festival ID from metadata is displayed
func TestFormatFestivalDetails_DisplaysMetadataID(t *testing.T) {
	tests := []struct {
		name           string
		festival       *FestivalInfo
		expectedOutput []string
		notExpected    []string
	}{
		{
			name: "displays festival ID prominently",
			festival: &FestivalInfo{
				ID:         "my-project_GU0001",
				MetadataID: "GU0001",
				Name:       "my-project",
				Status:     "active",
				Path:       "/path/to/my-project_GU0001",
			},
			expectedOutput: []string{"ID GU0001"},
		},
		{
			name: "handles legacy festival without metadata ID",
			festival: &FestivalInfo{
				ID:         "old-festival",
				MetadataID: "", // No metadata ID
				Name:       "old-festival",
				Status:     "active",
				Path:       "/path/to/old-festival",
			},
			expectedOutput: []string{"No ID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatFestivalDetails(tt.festival, false, "")

			for _, expected := range tt.expectedOutput {
				if !contains(output, expected) {
					t.Errorf("Output should contain %q, got:\n%s", expected, output)
				}
			}

			for _, notExpected := range tt.notExpected {
				if contains(output, notExpected) {
					t.Errorf("Output should NOT contain %q, got:\n%s", notExpected, output)
				}
			}
		})
	}
}

// TestFormatNodeReference tests the node reference format
func TestFormatNodeReference(t *testing.T) {
	tests := []struct {
		festivalID string
		phase      int
		sequence   int
		task       int
		expected   string
	}{
		{"GU0001", 1, 1, 1, "GU0001:P001.S01.T01"},
		{"GU0001", 12, 5, 99, "GU0001:P012.S05.T99"},
		{"FN0042", 2, 3, 4, "FN0042:P002.S03.T04"},
		{"", 1, 1, 1, ""}, // No ID, no reference
	}

	for _, tt := range tests {
		result := FormatNodeReference(tt.festivalID, tt.phase, tt.sequence, tt.task)
		if result != tt.expected {
			t.Errorf("FormatNodeReference(%q, %d, %d, %d) = %q, want %q",
				tt.festivalID, tt.phase, tt.sequence, tt.task, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	if substr == "" {
		return false
	}
	return strings.Contains(stripANSI(s), substr)
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func TestFormatFestivalList_ShowsTokenCount(t *testing.T) {
	festivals := []*FestivalInfo{
		{Name: "fest1", Path: "/path/to/fest1", Stats: &FestivalStats{Progress: 50}},
	}
	tokenMap := map[string]int{"/path/to/fest1": 1234}

	output := FormatFestivalList("active", festivals, tokenMap)
	if !contains(output, "1.2k tokens") {
		t.Errorf("expected token count in output, got: %s", output)
	}
	if !contains(output, "fest1") {
		t.Error("expected festival name in output")
	}
}

func TestFormatFestivalList_NilTokenMapOmitsTokens(t *testing.T) {
	festivals := []*FestivalInfo{
		{Name: "fest1", Stats: &FestivalStats{Progress: 50}},
	}
	output := FormatFestivalList("active", festivals, nil)
	if contains(output, "tokens") {
		t.Errorf("nil token map should not render tokens: %s", output)
	}
}

func TestFormatFestivalList_ZeroTokensOmitted(t *testing.T) {
	festivals := []*FestivalInfo{
		{Name: "fest1", Path: "/path/to/fest1", Stats: &FestivalStats{Progress: 50}},
	}
	tokenMap := map[string]int{"/path/to/fest1": 0}
	output := FormatFestivalList("active", festivals, tokenMap)
	if contains(output, "tokens") {
		t.Errorf("zero token count should not render tokens: %s", output)
	}
}

func TestFormatFestivalListWithProgress_ShowsTokenCount(t *testing.T) {
	festivals := []*FestivalInfo{
		{Name: "fest1", Path: "/path/to/fest1", Stats: &FestivalStats{Progress: 50}},
	}
	tokenMap := map[string]int{"/path/to/fest1": 5000}

	output := FormatFestivalListWithProgress("active", festivals, nil, tokenMap)
	if !contains(output, "5.0k tokens") {
		t.Errorf("expected token count in progress output, got: %s", output)
	}
}

func TestFormatTokenSuffix(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		tokenMap  map[string]int
		wantEmpty bool
		wantText  string
	}{
		{"nil map", "/path", nil, true, ""},
		{"path not in map", "/path", map[string]int{"/other": 100}, true, ""},
		{"zero tokens", "/path", map[string]int{"/path": 0}, true, ""},
		{"positive tokens", "/path", map[string]int{"/path": 1234}, false, "1.2k tokens"},
		{"large count", "/path", map[string]int{"/path": 50000}, false, "50k tokens"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTokenSuffix(tt.path, tt.tokenMap)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty suffix, got %q", got)
				}
				return
			}
			if !contains(got, tt.wantText) {
				t.Errorf("expected %q in suffix, got %q", tt.wantText, got)
			}
		})
	}
}
