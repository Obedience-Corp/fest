package explore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/id"
)

func TestDetectFestivalPath(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantHit bool
	}{
		{
			name: "in festival root with FESTIVAL_GOAL.md",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				festDir := filepath.Join(dir, "my-fest")
				_ = os.MkdirAll(festDir, 0755)
				_ = os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0644)
				return festDir
			},
			wantHit: true,
		},
		{
			name: "in festival root with fest.yaml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				festDir := filepath.Join(dir, "my-fest")
				_ = os.MkdirAll(festDir, 0755)
				_ = os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte("name: test\n"), 0644)
				return festDir
			},
			wantHit: true,
		},
		{
			name: "in phase directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				festDir := filepath.Join(dir, "my-fest")
				phaseDir := filepath.Join(festDir, "001_PLANNING")
				_ = os.MkdirAll(phaseDir, 0755)
				_ = os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0644)
				return phaseDir
			},
			wantHit: true,
		},
		{
			name: "in sequence directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				festDir := filepath.Join(dir, "my-fest")
				seqDir := filepath.Join(festDir, "001_PLANNING", "01_requirements")
				_ = os.MkdirAll(seqDir, 0755)
				_ = os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0644)
				return seqDir
			},
			wantHit: true,
		},
		{
			name: "outside any festival",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantHit: false,
		},
		{
			name: "deeply nested outside festival",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				deep := filepath.Join(dir, "a", "b", "c", "d")
				_ = os.MkdirAll(deep, 0755)
				return deep
			},
			wantHit: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := tt.setup(t)
			got := detectFestivalPath(cwd)
			if tt.wantHit && got == "" {
				t.Error("expected festival path, got empty")
			}
			if !tt.wantHit && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
		})
	}
}

func TestDetectFestivalPathReturnsRoot(t *testing.T) {
	dir := t.TempDir()
	festDir := filepath.Join(dir, "my-fest")
	seqDir := filepath.Join(festDir, "001_PHASE", "01_seq")
	_ = os.MkdirAll(seqDir, 0755)
	_ = os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0644)

	got := detectFestivalPath(seqDir)
	if got != festDir {
		t.Errorf("expected festival root %q, got %q", festDir, got)
	}
}

func TestStatusResolution(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"completed", "dungeon/completed"},
		{"archived", "dungeon/archived"},
		{"someday", "dungeon/someday"},
		{"active", "active"},
		{"planning", "planning"},
		{"ready", "ready"},
		{"ritual", "ritual"},
		{"dungeon", "dungeon"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := id.ResolveStatusPath(tt.input)
			if got != tt.want {
				t.Errorf("ResolveStatusPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewExploreCommandStructure(t *testing.T) {
	cmd := NewExploreCommand()

	if cmd.Use != "explore [status]" {
		t.Errorf("expected Use 'explore [status]', got %q", cmd.Use)
	}

	// Check --json flag exists
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag on explore command")
	}

	// Check subcommands exist
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	for _, expected := range []string{"active", "planning", "completed", "dungeon"} {
		if !subNames[expected] {
			t.Errorf("expected subcommand %q", expected)
		}
	}
}

func TestSubcommandHasJSONFlag(t *testing.T) {
	cmd := NewExploreCommand()

	for _, sub := range cmd.Commands() {
		jsonFlag := sub.Flags().Lookup("json")
		if jsonFlag == nil {
			t.Errorf("subcommand %q missing --json flag", sub.Use)
		}
	}
}
