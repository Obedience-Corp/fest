package list

import (
	"strings"
	"testing"
)

func TestFormatAllHumanEmpty(t *testing.T) {
	out := formatAllHuman(nil, nil, nil, 0, false)
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
