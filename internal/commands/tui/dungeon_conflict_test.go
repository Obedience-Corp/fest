package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHierarchySelector_ListFestivals_DungeonConflict(t *testing.T) {
	tests := []struct {
		name         string
		dungeonDirs  []string
		filterActive bool
		wantErr      bool
	}{
		{
			name:        "both spellings exist errors",
			dungeonDirs: []string{"dungeon", ".dungeon"},
			wantErr:     true,
		},
		{
			name:        "only visible spelling exists",
			dungeonDirs: []string{"dungeon"},
			wantErr:     false,
		},
		{
			name:        "only hidden spelling exists",
			dungeonDirs: []string{".dungeon"},
			wantErr:     false,
		},
		{
			name:        "neither spelling exists",
			dungeonDirs: nil,
			wantErr:     false,
		},
		{
			name:         "both spellings exist but active-only selector is unaffected",
			dungeonDirs:  []string{"dungeon", ".dungeon"},
			filterActive: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalsRoot := t.TempDir()
			for _, dir := range tt.dungeonDirs {
				if err := os.MkdirAll(filepath.Join(festivalsRoot, dir, "completed"), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			if err := os.MkdirAll(filepath.Join(festivalsRoot, "active"), 0755); err != nil {
				t.Fatalf("setup: %v", err)
			}

			h := NewHierarchySelector(festivalsRoot, HierarchyConfig{FilterActive: tt.filterActive})
			_, err := h.ListFestivals(context.Background())

			if tt.wantErr && err == nil {
				t.Fatal("ListFestivals() = nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ListFestivals() = %v, want nil", err)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), "dungeon/") || !strings.Contains(err.Error(), ".dungeon/") {
					t.Errorf("error should name both spellings, got: %v", err)
				}
				if !strings.Contains(err.Error(), "camp dungeon migrate") {
					t.Errorf("error should hint at the migration command, got: %v", err)
				}
			}
		})
	}
}

func TestFestivalSelector_loadFestivals_DungeonConflict(t *testing.T) {
	tests := []struct {
		name           string
		dungeonDirs    []string
		filterByStatus []string
		wantErr        bool
	}{
		{
			name:        "both spellings exist errors on default all-statuses config",
			dungeonDirs: []string{"dungeon", ".dungeon"},
			wantErr:     true,
		},
		{
			name:        "only visible spelling exists",
			dungeonDirs: []string{"dungeon"},
			wantErr:     false,
		},
		{
			name:        "neither spelling exists",
			dungeonDirs: nil,
			wantErr:     false,
		},
		{
			name:           "both spellings exist but an active-only filter is unaffected",
			dungeonDirs:    []string{"dungeon", ".dungeon"},
			filterByStatus: []string{"active"},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalsRoot := t.TempDir()
			for _, dir := range tt.dungeonDirs {
				if err := os.MkdirAll(filepath.Join(festivalsRoot, dir, "completed"), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			if err := os.MkdirAll(filepath.Join(festivalsRoot, "active"), 0755); err != nil {
				t.Fatalf("setup: %v", err)
			}

			config := DefaultSelectorConfig()
			config.FilterByStatus = tt.filterByStatus
			s := NewFestivalSelector(festivalsRoot, config)
			err := s.loadFestivals(context.Background())

			if tt.wantErr && err == nil {
				t.Fatal("loadFestivals() = nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("loadFestivals() = %v, want nil", err)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), "dungeon/") || !strings.Contains(err.Error(), ".dungeon/") {
					t.Errorf("error should name both spellings, got: %v", err)
				}
				if !strings.Contains(err.Error(), "camp dungeon migrate") {
					t.Errorf("error should hint at the migration command, got: %v", err)
				}
			}
		})
	}
}
