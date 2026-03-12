package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// ValidateAutoLink checks that implementation sequences declare a valid fest_working_dir.
// It uses the festival's AutoLinkConfig to determine which phase types require the field
// and whether to verify path existence on disk.
func ValidateAutoLink(ctx context.Context, festivalPath string, cfg *config.FestivalConfig) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !cfg.AutoLink.Enabled {
		return nil, nil
	}

	requiredPhases := make(map[string]bool, len(cfg.AutoLink.RequireOnPhases))
	for _, pt := range cfg.AutoLink.RequireOnPhases {
		requiredPhases[pt] = true
	}

	campaignRoot := resolveCampaignRoot(festivalPath)

	parser := festival.NewParser()
	phases, err := parser.ParsePhases(ctx, festivalPath)
	if err != nil {
		return nil, err
	}

	var issues []Issue

	for _, phase := range phases {
		if err := ctx.Err(); err != nil {
			return issues, err
		}

		phaseType := detectPhaseType(phase.Path)
		isRequired := requiredPhases[phaseType]

		sequences, err := parser.ParseSequences(ctx, phase.Path)
		if err != nil {
			continue
		}

		for _, seq := range sequences {
			seqGoalPath := filepath.Join(seq.Path, "SEQUENCE_GOAL.md")
			content, err := os.ReadFile(seqGoalPath)
			if err != nil {
				continue
			}

			fm, _, err := frontmatter.Parse(content)
			if err != nil || fm == nil {
				continue
			}

			workingDir := strings.TrimSpace(fm.WorkingDir)

			if workingDir == "" {
				if isRequired {
					issues = append(issues, Issue{
						Level:   LevelError,
						Code:    CodeAutoLinkMissingWorkingDir,
						Path:    seqGoalPath,
						Message: fmt.Sprintf("sequence %q: missing fest_working_dir in GOAL — set the target project directory (relative to campaign root)", seq.FullName),
						Fix:     "Add fest_working_dir: \"projects/your-project\" to the SEQUENCE_GOAL.md frontmatter",
					})
				}
				continue
			}

			if !isRequired {
				issues = append(issues, Issue{
					Level:   LevelInfo,
					Code:    CodeAutoLinkUnrequiredSet,
					Path:    seqGoalPath,
					Message: fmt.Sprintf("sequence %q: fest_working_dir is set on %s phase — only validated on: %s", seq.FullName, phaseType, strings.Join(cfg.AutoLink.RequireOnPhases, ", ")),
				})
			}

			normalized, normErr := pathutil.NormalizeWorkingDir(workingDir)
			if normErr != nil {
				code := CodeAutoLinkAbsolutePath
				if strings.Contains(workingDir, "..") {
					code = CodeAutoLinkPathTraversal
				}
				issues = append(issues, Issue{
					Level:   LevelError,
					Code:    code,
					Path:    seqGoalPath,
					Message: fmt.Sprintf("sequence %q: %s", seq.FullName, normErr.Error()),
				})
				continue
			}

			if cfg.AutoLink.ValidatePathExists && campaignRoot != "" {
				absPath := filepath.Join(campaignRoot, normalized)
				info, statErr := os.Stat(absPath)
				if statErr != nil {
					issues = append(issues, Issue{
						Level:   LevelError,
						Code:    CodeAutoLinkPathNotFound,
						Path:    seqGoalPath,
						Message: fmt.Sprintf("sequence %q: fest_working_dir %q not found relative to campaign root %q", seq.FullName, normalized, campaignRoot),
						Fix:     fmt.Sprintf("Verify the path exists: %s", absPath),
					})
				} else if !info.IsDir() {
					issues = append(issues, Issue{
						Level:   LevelError,
						Code:    CodeAutoLinkPathNotDir,
						Path:    seqGoalPath,
						Message: fmt.Sprintf("sequence %q: fest_working_dir %q is a file, expected a directory", seq.FullName, normalized),
					})
				}
			}
		}
	}

	return issues, nil
}

// detectPhaseType reads the phase type from PHASE_GOAL.md frontmatter.
// Falls back to "implementation" if the frontmatter cannot be read.
func detectPhaseType(phasePath string) string {
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return string(frontmatter.PhaseTypeImplementation)
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil || fm.PhaseType == "" {
		return string(frontmatter.PhaseTypeImplementation)
	}
	return string(fm.PhaseType)
}

// resolveCampaignRoot finds the campaign root directory by walking up from festivalPath.
// Returns empty string if no campaign root is found (validation degrades gracefully).
func resolveCampaignRoot(festivalPath string) string {
	ctx := context.Background()

	// Try campaign detection first (handles CAMP_ROOT env var)
	root, err := workspace.DetectCampaign(ctx, festivalPath)
	if err == nil {
		return root
	}

	// Fallback: walk up to find the parent of festivals/ directory
	dir := festivalPath
	for dir != "" && dir != "/" {
		if filepath.Base(dir) == "festivals" {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}
