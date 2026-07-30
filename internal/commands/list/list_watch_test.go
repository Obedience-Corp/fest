package list

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func TestFormatAllHumanEmpty(t *testing.T) {
	out := formatAllHuman(nil, nil, nil, nil, 0, false)
	if !strings.Contains(out, "No festivals found") {
		t.Fatalf("expected empty-board message, got %q", out)
	}
	if !strings.Contains(out, "fest create festival") {
		t.Fatalf("expected create hint, got %q", out)
	}
}

func TestFormatDungeonHumanEmpty(t *testing.T) {
	out := formatDungeonHuman(nil, nil, nil, 0, false)
	if !strings.Contains(out, "No festivals in dungeon") {
		t.Fatalf("expected empty dungeon message, got %q", out)
	}
}

// A campaign with no residents must render exactly what the pre-rail binary
// rendered: no residents header, no separator, no count.
func TestFormatStatusHuman_NoResidentsIsUnchanged(t *testing.T) {
	withResidents := formatStatusHuman("active", nil, nil, nil, false)
	if strings.Contains(strings.ToLower(withResidents), "resident") {
		t.Errorf("resident-free output mentions residents: %q", withResidents)
	}
}

func TestFormatStatusHuman_ResidentsAppendAfterFestivals(t *testing.T) {
	residents := []*show.ResidentCard{
		{Name: "alpha", Title: "Alpha", Type: "design"},
		{Name: "beta", Title: "Beta", Type: "explore"},
	}
	out := formatStatusHuman("active", nil, residents, nil, false)

	if !strings.Contains(out, "ACTIVE Residents (2)") {
		t.Errorf("missing residents header: %q", out)
	}
	for _, want := range []string{"alpha", "beta", "[design]", "[explore]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	// Festivals first, residents after.
	if fi, ri := strings.Index(out, "ACTIVE Festivals"), strings.Index(out, "ACTIVE Residents"); fi < 0 || ri < fi {
		t.Errorf("residents block should follow the festival block: %q", out)
	}
}

func TestFormatAllHuman_NoResidentsKeepsLegacyPath(t *testing.T) {
	// With no residents the empty-board message must still appear, unchanged.
	out := formatAllHuman(nil, nil, nil, nil, 0, false)
	if !strings.Contains(out, "No festivals found") {
		t.Errorf("expected the legacy empty message, got %q", out)
	}
	if strings.Contains(strings.ToLower(out), "resident") {
		t.Errorf("resident-free output mentions residents: %q", out)
	}
}

func TestFormatAllHuman_ResidentOnlyStageStillRenders(t *testing.T) {
	residents := map[string][]*show.ResidentCard{
		"active": {{Name: "solo", Title: "Solo", Type: "design"}},
	}
	// totalCount 0: no festivals at all, only a resident. It must not fall into
	// the "No festivals found" branch, or the resident would be invisible.
	out := formatAllHuman(nil, residents, []string{"active"}, nil, 0, false)
	if strings.Contains(out, "No festivals found") {
		t.Fatalf("a resident-only campaign must not report an empty board: %q", out)
	}
	if !strings.Contains(out, "solo") {
		t.Errorf("resident missing from output: %q", out)
	}
}
