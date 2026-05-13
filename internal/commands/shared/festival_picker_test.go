package shared

import "testing"

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
