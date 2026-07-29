package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// festivalDraft holds human create wizard answers shared by charm and fallback.
type festivalDraft struct {
	TypeName    string
	Name        string
	Goal        string
	Project     string
	ProjectMode string // pick | path | skip
	Seed        string
	Tags        string
}

const (
	projectModePick = "pick"
	projectModePath = "path"
	projectModeSkip = "skip"
)

func defaultTypeName(cfg *types.FestivalTypesConfig) string {
	if d := cfg.GetDefaultType(); d != nil {
		return d.Name
	}
	if len(cfg.Types) == 0 {
		return ""
	}
	return cfg.Types[0].Name
}

func typeByName(cfg *types.FestivalTypesConfig, name string) *types.FestivalType {
	ft, err := cfg.GetFestivalType(name)
	if err != nil {
		return cfg.GetDefaultType()
	}
	return ft
}

func festivalTypeDest(ft *types.FestivalType) string {
	if ft != nil && ft.Name == "ritual" {
		return "ritual"
	}
	return "planning"
}

// festivalTypeSupportsSeed mirrors festival.ingestAutoPhaseID: only an
// auto-scaffolded phase with Type == "ingest" can receive --seed.
func festivalTypeSupportsSeed(ft *types.FestivalType) bool {
	if ft == nil || ft.SkipIngestion {
		return false
	}
	for _, p := range ft.GetAutoPhases() {
		if p.Type == "ingest" {
			return true
		}
	}
	return false
}

// festivalTypeOptionLabel is a short select key (details go in notes/descriptions).
func festivalTypeOptionLabel(ft types.FestivalType) string {
	name := ft.Name
	if ft.Default {
		name += "*"
	}
	return name
}

func typesHelpNote(cfg *types.FestivalTypesConfig) string {
	var b strings.Builder
	b.WriteString("Types from festival_types.yaml. * = default.\n")
	for _, t := range cfg.Types {
		auto := t.GetAutoPhases()
		autoStr := "none"
		if len(auto) > 0 {
			names := make([]string, len(auto))
			for i, p := range auto {
				names[i] = p.Name
			}
			autoStr = strings.Join(names, "→")
		}
		fmt.Fprintf(&b, "· %s — %s · auto:%s · %s/\n",
			festivalTypeOptionLabel(t), t.Description, autoStr, festivalTypeDest(&t))
	}
	return strings.TrimRight(b.String(), "\n")
}

func autoPhaseSummary(ft *types.FestivalType) string {
	if ft == nil {
		return "(none)"
	}
	auto := ft.GetAutoPhases()
	if len(auto) == 0 {
		return "(none)"
	}
	names := make([]string, len(auto))
	for i, p := range auto {
		names[i] = p.Name
	}
	return strings.Join(names, " → ")
}

// nextStepAfterProject returns the wizard step after project (4=seed, 5=details).
// Clears draft.Seed when the type cannot be seeded.
func nextStepAfterProject(cfg *types.FestivalTypesConfig, d *festivalDraft) int {
	ft := typeByName(cfg, d.TypeName)
	if festivalTypeSupportsSeed(ft) {
		return 4
	}
	d.Seed = ""
	return 5
}

func buildFestivalConfirmSummary(cfg *types.FestivalTypesConfig, d *festivalDraft) string {
	ft := typeByName(cfg, d.TypeName)
	auto := ft.GetAutoPhases()
	autoStr := "(none)"
	if len(auto) > 0 {
		parts := make([]string, len(auto))
		for i, p := range auto {
			// Avoid underscores in Note text (markdown italics).
			parts[i] = fmt.Sprintf("%03d-%s", i+1, p.Name)
		}
		autoStr = strings.Join(parts, ", ") + " (auto)"
	}
	proj := d.Project
	if proj == "" {
		proj = "(none)"
	}
	seed := "no"
	if s := strings.TrimSpace(d.Seed); s != "" {
		seed = fmt.Sprintf("yes (%d chars)", len(s))
	}
	tags := d.Tags
	if tags == "" {
		tags = "(none)"
	}
	goal := d.Goal
	if goal == "" {
		goal = "(empty — agents plan better with a goal)"
	}
	goal = truncateRunes(goal, 80)
	return fmt.Sprintf(
		"Name:     %s\nType:     %s\nDest:     festivals/%s/\nPhases:   %s\nProject:  %s\nSeed:     %s\nTags:     %s\nGoal:     %s",
		d.Name, ft.Name, festivalTypeDest(ft), autoStr, proj, seed, tags, goal,
	)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// trimTagList splits comma tags and trims whitespace (matches create parseTags).
func trimTagList(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return nil
	}
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildFestivalVars builds template vars for create, including optional extras.
func buildFestivalVars(d *festivalDraft, extra map[string]interface{}) map[string]interface{} {
	vars := map[string]interface{}{
		"festival_name": d.Name,
		"festival_goal": d.Goal,
	}
	if tags := trimTagList(d.Tags); len(tags) > 0 {
		vars["festival_tags"] = tags
	}
	for k, v := range extra {
		if _, exists := vars[k]; !exists {
			vars[k] = v
		}
	}
	return vars
}

// resolveProjectMode picks the project step default from the draft.
func resolveProjectMode(d *festivalDraft, projects []string) string {
	if d.ProjectMode == projectModePick || d.ProjectMode == projectModePath || d.ProjectMode == projectModeSkip {
		return d.ProjectMode
	}
	if d.Project == "" {
		if len(projects) > 0 {
			return projectModePick
		}
		return projectModeSkip
	}
	for _, p := range projects {
		if p == d.Project {
			return projectModePick
		}
	}
	return projectModePath
}

// projectPick is a campaign project path with a human label for the picker.
type projectPick struct {
	Label string // display (e.g. "camp" or "monorepo@pkg")
	Path  string // filesystem path relative to campaign root
}

// listCampaignProjects returns projects/* paths (and monorepo submodules) under
// the campaign root, matching navigation's monorepo expansion.
func listCampaignProjects(ctx context.Context) []projectPick {
	root, err := workspace.DetectCampaign(ctx, "")
	if err != nil || root == "" {
		return nil
	}
	projectsDir := filepath.Join(root, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}
	var out []projectPick
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "worktrees" {
			continue
		}
		rel := filepath.Join("projects", name)
		out = append(out, projectPick{Label: name, Path: rel})
		// Expand monorepo submodules like go-link (label name@sub, path projects/name/sub).
		for _, sub := range listGitSubmodulePaths(filepath.Join(projectsDir, name)) {
			out = append(out, projectPick{
				Label: name + "@" + sub,
				Path:  filepath.Join("projects", name, sub),
			})
		}
	}
	return out
}

func projectPaths(picks []projectPick) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Path
	}
	return out
}

func listGitSubmodulePaths(projectDir string) []string {
	cmd := exec.Command("git", "-C", projectDir, "config", "-f", ".gitmodules", "--list")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, ".path=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[1] != "" {
			paths = append(paths, parts[1])
		}
	}
	return paths
}
