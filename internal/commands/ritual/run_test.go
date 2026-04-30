package ritual

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteRitualName(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsDir := filepath.Join(tmpDir, "festivals")
	if err := os.MkdirAll(filepath.Join(festivalsDir, ".festival"), 0755); err != nil {
		t.Fatalf("mkdir .festival: %v", err)
	}
	ritualDir := filepath.Join(festivalsDir, "ritual")
	if err := os.MkdirAll(ritualDir, 0755); err != nil {
		t.Fatalf("mkdir ritual: %v", err)
	}
	rituals := []string{
		"daily-cleanup-RI-DC0001",
		"weekly-review-RI-WR0001",
		"monthly-audit-RI-MA0001",
	}
	for _, name := range rituals {
		if err := os.MkdirAll(filepath.Join(ritualDir, name), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(ritualDir, "not-a-festival"), 0755); err != nil {
		t.Fatalf("mkdir non-festival: %v", err)
	}

	chdir(t, tmpDir)

	tests := []struct {
		name       string
		toComplete string
		want       []string
		wantEmpty  bool
	}{
		{
			name:       "empty input returns all rituals",
			toComplete: "",
			want:       rituals,
		},
		{
			name:       "prefix match",
			toComplete: "daily",
			want:       []string{"daily-cleanup-RI-DC0001"},
		},
		{
			name:       "case-insensitive substring match",
			toComplete: "REVIEW",
			want:       []string{"weekly-review-RI-WR0001"},
		},
		{
			name:       "ID suffix substring match",
			toComplete: "MA0001",
			want:       []string{"monthly-audit-RI-MA0001"},
		},
		{
			name:       "no matches returns empty",
			toComplete: "nonexistent-ritual",
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, directive := CompleteRitualName(nil, nil, tt.toComplete)
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
			}
			if tt.wantEmpty {
				if len(got) != 0 {
					t.Errorf("got %v, want empty", got)
				}
				return
			}
			if !sameElements(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompleteRitualName_NoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	got, directive := CompleteRitualName(nil, nil, "")
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestCompleteRitualName_MissingRitualDir(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "festivals", ".festival"), 0755); err != nil {
		t.Fatalf("mkdir .festival: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "festivals", "active"), 0755); err != nil {
		t.Fatalf("mkdir active: %v", err)
	}
	chdir(t, tmpDir)

	got, directive := CompleteRitualName(nil, nil, "")
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}

func sameElements(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	gotSet := make(map[string]int, len(got))
	for _, g := range got {
		gotSet[g]++
	}
	for _, w := range want {
		if gotSet[w] == 0 {
			return false
		}
		gotSet[w]--
	}
	return true
}
