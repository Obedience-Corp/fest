package validator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
)

// setupAutoLinkFestival creates a temp campaign with a festival structure for auto-link testing.
// Returns the festival path. Creates .campaign/ marker at the campaign root.
func setupAutoLinkFestival(t *testing.T, structure map[string]string) string {
	t.Helper()
	campaignRoot := t.TempDir()

	// Create .campaign marker directory
	if err := os.MkdirAll(filepath.Join(campaignRoot, ".campaign"), 0755); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "test-festival")
	for relPath, content := range structure {
		fullPath := filepath.Join(festivalPath, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return festivalPath
}

func TestValidateAutoLink(t *testing.T) {
	implPhaseGoal := "---\nfest_type: phase\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	planningPhaseGoal := "---\nfest_type: phase\nfest_phase_type: planning\n---\n# Phase Goal\n"

	enabledCfg := &config.FestivalConfig{
		AutoLink: config.AutoLinkConfig{
			Enabled:            true,
			RequireOnPhases:    []string{"implementation"},
			ValidatePathExists: false,
		},
	}

	enabledWithPathCheck := &config.FestivalConfig{
		AutoLink: config.AutoLinkConfig{
			Enabled:            true,
			RequireOnPhases:    []string{"implementation"},
			ValidatePathExists: true,
		},
	}

	disabledCfg := &config.FestivalConfig{
		AutoLink: config.AutoLinkConfig{
			Enabled: false,
		},
	}

	tests := []struct {
		name       string
		structure  map[string]string
		cfg        *config.FestivalConfig
		wantCode   string
		wantCount  int
		wantLevel  string
		setupExtra func(t *testing.T, festivalPath string) // Extra setup after festival creation
	}{
		{
			name: "disabled auto-link returns no issues",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\n---\n# Goal\n",
			},
			cfg:       disabledCfg,
			wantCount: 0,
		},
		{
			name: "missing working_dir on implementation sequence",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\n---\n# Goal\n",
			},
			cfg:       enabledCfg,
			wantCode:  CodeAutoLinkMissingWorkingDir,
			wantCount: 1,
			wantLevel: LevelError,
		},
		{
			name: "absolute path produces error",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: /absolute/path\n---\n# Goal\n",
			},
			cfg:       enabledCfg,
			wantCode:  CodeAutoLinkAbsolutePath,
			wantCount: 1,
			wantLevel: LevelError,
		},
		{
			name: "path traversal produces error",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: ../../../etc\n---\n# Goal\n",
			},
			cfg:       enabledCfg,
			wantCode:  CodeAutoLinkPathTraversal,
			wantCount: 1,
			wantLevel: LevelError,
		},
		{
			name: "non-existent path with validate_path_exists",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: projects/nonexistent\n---\n# Goal\n",
			},
			cfg:       enabledWithPathCheck,
			wantCode:  CodeAutoLinkPathNotFound,
			wantCount: 1,
			wantLevel: LevelError,
		},
		{
			name: "path pointing to file produces error",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: projects/afile\n---\n# Goal\n",
			},
			cfg: enabledWithPathCheck,
			setupExtra: func(t *testing.T, festivalPath string) {
				// Create a file (not dir) at the target path in campaign root
				campaignRoot := resolveCampaignRoot(context.Background(), festivalPath)
				filePath := filepath.Join(campaignRoot, "projects", "afile")
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filePath, []byte("not a dir"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantCode:  CodeAutoLinkPathNotDir,
			wantCount: 1,
			wantLevel: LevelError,
		},
		{
			name: "valid path on implementation sequence - no issues",
			structure: map[string]string{
				"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
				"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: projects/fest\n---\n# Goal\n",
			},
			cfg: enabledWithPathCheck,
			setupExtra: func(t *testing.T, festivalPath string) {
				campaignRoot := resolveCampaignRoot(context.Background(), festivalPath)
				if err := os.MkdirAll(filepath.Join(campaignRoot, "projects", "fest"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantCount: 0,
		},
		{
			name: "missing working_dir on non-required phase is ok",
			structure: map[string]string{
				"001_PLANNING/PHASE_GOAL.md":           planningPhaseGoal,
				"001_PLANNING/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\n---\n# Goal\n",
			},
			cfg:       enabledCfg,
			wantCount: 0,
		},
		{
			name: "working_dir set on non-required phase produces info",
			structure: map[string]string{
				"001_PLANNING/PHASE_GOAL.md":           planningPhaseGoal,
				"001_PLANNING/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\nfest_working_dir: projects/fest\n---\n# Goal\n",
			},
			cfg:       enabledCfg,
			wantCode:  CodeAutoLinkUnrequiredSet,
			wantCount: 1,
			wantLevel: LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalPath := setupAutoLinkFestival(t, tt.structure)
			if tt.setupExtra != nil {
				tt.setupExtra(t, festivalPath)
			}

			issues, err := ValidateAutoLink(context.Background(), festivalPath, tt.cfg)
			if err != nil {
				t.Fatalf("ValidateAutoLink() unexpected error: %v", err)
			}

			if tt.wantCount == 0 {
				if len(issues) != 0 {
					t.Errorf("expected no issues, got %d: %+v", len(issues), issues)
				}
				return
			}

			count := countIssuesByCode(issues, tt.wantCode)
			if count != tt.wantCount {
				t.Errorf("expected %d issues with code %q, got %d (total issues: %+v)", tt.wantCount, tt.wantCode, count, issues)
			}

			if tt.wantLevel != "" {
				for _, iss := range issues {
					if iss.Code == tt.wantCode && iss.Level != tt.wantLevel {
						t.Errorf("issue %q expected level %q, got %q", iss.Code, tt.wantLevel, iss.Level)
					}
				}
			}
		})
	}
}

func TestValidateAutoLink_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.FestivalConfig{
		AutoLink: config.DefaultAutoLinkConfig(),
	}

	_, err := ValidateAutoLink(ctx, "/nonexistent", cfg)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestValidateAutoLink_ProjectPathFallbackRelative(t *testing.T) {
	implPhaseGoal := "---\nfest_type: phase\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	festivalPath := setupAutoLinkFestival(t, map[string]string{
		"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
		"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\n---\n# Goal\n",
	})

	campaignRoot := resolveCampaignRoot(context.Background(), festivalPath)
	targetPath := filepath.Join(campaignRoot, "projects", "fest")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.FestivalConfig{
		ProjectPath: "projects/fest",
		AutoLink: config.AutoLinkConfig{
			Enabled:            true,
			RequireOnPhases:    []string{"implementation"},
			ValidatePathExists: true,
		},
	}

	issues, err := ValidateAutoLink(context.Background(), festivalPath, cfg)
	if err != nil {
		t.Fatalf("ValidateAutoLink() unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateAutoLink_ProjectPathFallbackAbsolute(t *testing.T) {
	implPhaseGoal := "---\nfest_type: phase\nfest_phase_type: implementation\n---\n# Phase Goal\n"
	festivalPath := setupAutoLinkFestival(t, map[string]string{
		"001_IMPLEMENT/PHASE_GOAL.md":           implPhaseGoal,
		"001_IMPLEMENT/01_seq/SEQUENCE_GOAL.md": "---\nfest_type: sequence\n---\n# Goal\n",
	})

	campaignRoot := resolveCampaignRoot(context.Background(), festivalPath)
	targetPath := filepath.Join(campaignRoot, "projects", "fest")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.FestivalConfig{
		ProjectPath: targetPath,
		AutoLink: config.AutoLinkConfig{
			Enabled:            true,
			RequireOnPhases:    []string{"implementation"},
			ValidatePathExists: true,
		},
	}

	issues, err := ValidateAutoLink(context.Background(), festivalPath, cfg)
	if err != nil {
		t.Fatalf("ValidateAutoLink() unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}
