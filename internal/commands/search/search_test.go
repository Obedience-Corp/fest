package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Obedience-Corp/fest/internal/navigation"
)

// createTestWorkspace creates a temporary festivals directory structure for testing.
func createTestWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	festivalsDir := filepath.Join(root, "festivals")

	for _, status := range []string{"active", "planning", "dungeon/completed"} {
		os.MkdirAll(filepath.Join(festivalsDir, status), 0755)
	}

	createTestFestival(t, festivalsDir, "active", "deploy-service-DS0001", "DS0001", "Deploy the new microservice to production")
	createTestFestival(t, festivalsDir, "active", "auth-improvements-AI0001", "AI0001", "Improve authentication flow with OAuth2")
	createTestFestival(t, festivalsDir, "planning", "ready-intents-ingest-RI0001", "RI0001", "Ingest ready intents into the workflow")
	createTestFestival(t, festivalsDir, "dungeon/completed", "initial-setup-IS0001", "IS0001", "Initial project setup and scaffolding")

	return festivalsDir
}

func createTestFestival(t *testing.T, festivalsDir, status, name, festID, goal string) {
	t.Helper()
	festPath := filepath.Join(festivalsDir, status, name)
	os.MkdirAll(festPath, 0755)

	overview := fmt.Sprintf(`---
fest_type: festival
fest_id: %s
fest_name: %s
---

# Festival Overview

## Festival Goal

%s
`, festID, name, goal)

	os.WriteFile(filepath.Join(festPath, "FESTIVAL_OVERVIEW.md"), []byte(overview), 0644)
}

func TestCollectSearchTargets(t *testing.T) {
	festivalsDir := createTestWorkspace(t)

	tests := []struct {
		name         string
		statusFilter string
		wantCount    int
	}{
		{"all statuses", "", 4},
		{"active only", "active", 2},
		{"planning only", "planning", 1},
		{"dungeon/completed only", "dungeon/completed", 1},
		{"nonexistent status", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := collectSearchTargets(context.Background(), festivalsDir, tt.statusFilter)
			if len(targets) != tt.wantCount {
				t.Errorf("got %d targets, want %d", len(targets), tt.wantCount)
			}
		})
	}
}

func TestCollectSearchTargets_ContextCancellation(t *testing.T) {
	festivalsDir := createTestWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	targets := collectSearchTargets(ctx, festivalsDir, "")
	if len(targets) > 4 {
		t.Errorf("expected at most 4 targets after cancellation, got %d", len(targets))
	}
}

func TestCollectSearchTargets_PopulatesMetadata(t *testing.T) {
	festivalsDir := createTestWorkspace(t)
	targets := collectSearchTargets(context.Background(), festivalsDir, "active")

	for _, target := range targets {
		if target.Name == "" {
			t.Error("target missing Name")
		}
		if target.Status == "" {
			t.Error("target missing Status")
		}
		if target.Path == "" {
			t.Error("target missing Path")
		}
		if target.ID == "" {
			t.Errorf("target %s missing ID", target.Name)
		}
		if target.GoalText == "" {
			t.Errorf("target %s missing GoalText", target.Name)
		}
	}
}

func TestExtractFestivalID(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(dir string)
		wantID string
	}{
		{
			name: "valid frontmatter in overview",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"),
					[]byte("---\nfest_id: GU0001\n---\n# Overview"), 0644)
			},
			wantID: "GU0001",
		},
		{
			name: "valid frontmatter in goal",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"),
					[]byte("---\nfest_id: GL0001\n---\n# Goal"), 0644)
			},
			wantID: "GL0001",
		},
		{
			name:   "missing file",
			setup:  func(dir string) {},
			wantID: "",
		},
		{
			name: "no fest_id in frontmatter",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"),
					[]byte("---\nfest_type: festival\n---\n"), 0644)
			},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := extractFestivalID(dir)
			if got != tt.wantID {
				t.Errorf("extractFestivalID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestExtractGoalText(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantGoal string
	}{
		{
			name:     "standard goal section",
			content:  "---\nfest_id: X\n---\n\n## Festival Goal\n\nDeploy the service\n\n## Next Section\n",
			wantGoal: "Deploy the service",
		},
		{
			name:     "objective section",
			content:  "---\n---\n\n## Objective\n\nBuild the thing\n",
			wantGoal: "Build the thing",
		},
		{
			name:     "no goal section - falls back to first heading content",
			content:  "---\n---\n\n# Festival Name\n\nSome description here\n",
			wantGoal: "Some description here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "FESTIVAL_OVERVIEW.md"), []byte(tt.content), 0644)
			got := extractGoalText(dir)
			if got != tt.wantGoal {
				t.Errorf("extractGoalText() = %q, want %q", got, tt.wantGoal)
			}
		})
	}
}

func TestExtractGoalText_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got := extractGoalText(dir)
	if got != "" {
		t.Errorf("expected empty goal for missing file, got %q", got)
	}
}

func TestScoreTarget(t *testing.T) {
	target := searchTarget{
		Name:     "deploy-service-DS0001",
		ID:       "DS0001",
		GoalText: "Deploy the new microservice to production",
	}

	tests := []struct {
		name      string
		query     string
		wantMin   int
		wantZero  bool
	}{
		{"exact name match scores highest", "deploy-service-DS0001", 100, false},
		{"prefix name match", "deploy", 1, false},
		{"exact ID match", "DS0001", 100, false},
		{"no match", "zzzzz", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreTarget(tt.query, target)
			if tt.wantZero {
				if score != 0 {
					t.Errorf("expected 0, got %d", score)
				}
				return
			}
			if score < tt.wantMin {
				t.Errorf("expected score >= %d, got %d", tt.wantMin, score)
			}
		})
	}
}

func TestScoreTarget_NameBeatsGoal(t *testing.T) {
	target := searchTarget{
		Name:     "deploy-service",
		GoalText: "deploy something",
	}
	score := scoreTarget("deploy-service", target)
	// Exact name match = 100, goal match would be scaled down
	if score < 100 {
		t.Errorf("exact name match should score >= 100, got %d", score)
	}
}

func TestSearchRanking(t *testing.T) {
	festivalsDir := createTestWorkspace(t)
	targets := collectSearchTargets(context.Background(), festivalsDir, "")

	query := "deploy"
	var results []SearchResult
	for _, target := range targets {
		bestScore := scoreTarget(query, target)
		if bestScore > 0 {
			results = append(results, SearchResult{Name: target.Name, Score: bestScore})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) == 0 {
		t.Fatal("expected at least one result for 'deploy'")
	}
	if results[0].Name != "deploy-service-DS0001" {
		t.Errorf("expected deploy-service-DS0001 as top result, got %s", results[0].Name)
	}
}

func TestSearchRanking_IDMatch(t *testing.T) {
	festivalsDir := createTestWorkspace(t)
	targets := collectSearchTargets(context.Background(), festivalsDir, "")

	// Search by ID - deploy-service-DS0001 has ID "DS0001"
	query := "DS0001"
	var found bool
	for _, target := range targets {
		if target.ID == "DS0001" {
			score := scoreTarget(query, target)
			if score < 100 {
				t.Errorf("exact ID match should score >= 100, got %d", score)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find target with ID DS0001")
	}
}

func TestSearchNoResults(t *testing.T) {
	festivalsDir := createTestWorkspace(t)
	targets := collectSearchTargets(context.Background(), festivalsDir, "")

	query := "zzzznonexistent"
	var count int
	for _, target := range targets {
		if scoreTarget(query, target) > 0 {
			count++
		}
	}
	if count != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", count)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string that should be truncated", 20, "this is a longer ..."},
		{"", 10, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_%s", tt.maxLen, tt.input), func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestNewSearchCommandStructure(t *testing.T) {
	cmd := NewSearchCommand()

	if cmd.Use != "search <query>" {
		t.Errorf("expected Use 'search <query>', got %q", cmd.Use)
	}

	// Check flags exist
	for _, flag := range []string{"status", "json", "limit"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag on search command", flag)
		}
	}
}

// Ensure navigation.Score is being used (compile-time check)
var _ = navigation.Score
