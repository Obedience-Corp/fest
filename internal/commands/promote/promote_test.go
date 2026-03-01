package promote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/id"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from   string
		wantTo string
		wantOK bool
	}{
		{"planning", "ready", true},
		{"ready", "active", true},
		{"active", "completed", true},
		{"completed", "", false},
		{"dungeon", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.from, func(t *testing.T) {
			to, ok := validTransitions[tt.from]
			if ok != tt.wantOK {
				t.Errorf("validTransitions[%q]: got ok=%v, want ok=%v", tt.from, ok, tt.wantOK)
			}
			if to != tt.wantTo {
				t.Errorf("validTransitions[%q]: got %q, want %q", tt.from, to, tt.wantTo)
			}
		})
	}
}

func TestValidatePlannedToReady(t *testing.T) {
	t.Run("with goal file", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\nTest"), 0644)

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "planning",
		}

		err := validatePlannedToReady(festival)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("without goal file", func(t *testing.T) {
		dir := t.TempDir()

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "planning",
		}

		err := validatePlannedToReady(festival)
		if err == nil {
			t.Error("expected error for missing FESTIVAL_GOAL.md")
		}
	})
}

func TestValidateActiveToCompleted(t *testing.T) {
	t.Run("empty festival", func(t *testing.T) {
		dir := t.TempDir()

		festival := &show.FestivalInfo{
			Name:   "test-festival",
			Path:   dir,
			Status: "active",
		}

		err := validateActiveToCompleted(t.Context(), festival)
		if err == nil {
			t.Error("expected error for festival with no tasks")
		}
	})
}

func TestNewPromoteCommand(t *testing.T) {
	cmd := NewPromoteCommand()
	if cmd.Use != "promote" {
		t.Errorf("expected Use=%q, got %q", "promote", cmd.Use)
	}

	// Check flags exist
	flags := []string{"force", "json", "dungeon"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestDungeonFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		dungeon string
		wantErr bool
	}{
		{"valid completed", "completed", false},
		{"valid archived", "archived", false},
		{"valid someday", "someday", false},
		{"invalid status", "active", true},
		{"invalid arbitrary", "nonsense", true},
		{"invalid planning", "planning", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := id.ResolveStatusPath(tt.dungeon)
			isDungeon := strings.HasPrefix(resolved, "dungeon/")
			if isDungeon == tt.wantErr {
				t.Errorf("dungeon=%q: resolved=%q, isDungeon=%v, wantErr=%v",
					tt.dungeon, resolved, isDungeon, tt.wantErr)
			}
		})
	}
}
