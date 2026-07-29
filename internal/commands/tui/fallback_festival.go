//go:build no_charm

package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func tuiCreateFestival(ctx context.Context, display *ui.UI) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg, err := types.LoadFestivalTypesConfig(ctx)
	if err != nil {
		return err
	}
	if len(cfg.Types) == 0 {
		return errors.Validation("no festival types configured")
	}

	// Type
	typeNames := make([]string, len(cfg.Types))
	for i, t := range cfg.Types {
		label := t.Name
		if t.Default {
			label += " (default)"
		}
		auto := t.GetAutoPhases()
		if len(auto) > 0 {
			names := make([]string, len(auto))
			for j, p := range auto {
				names[j] = p.Name
			}
			label += " — " + strings.Join(names, "→")
		}
		typeNames[i] = label
	}
	tIdx := display.Choose("Festival type:", typeNames)
	if tIdx < 0 || tIdx >= len(cfg.Types) {
		tIdx = 0
		for i, t := range cfg.Types {
			if t.Default {
				tIdx = i
				break
			}
		}
	}
	ft := cfg.Types[tIdx]
	festType := ft.Name

	name := strings.TrimSpace(display.Prompt("Festival name"))
	if name == "" {
		return errors.Validation("festival name is required")
	}
	goal := strings.TrimSpace(display.PromptDefault("Festival goal", ""))
	tags := strings.TrimSpace(display.PromptDefault("Tags (comma-separated)", ""))

	// Project
	project := ""
	if display.Confirm("Link a project now?") {
		project = strings.TrimSpace(display.PromptDefault("Project path", ""))
	}

	// Seed (only if ingest supported)
	seed := ""
	if festivalTypeSupportsSeedFallback(&ft) {
		if display.Confirm("Seed ingest with starting material?") {
			seed = strings.TrimSpace(display.PromptDefault("Seed brief", ""))
		}
	}

	dest := "planning"
	if festType == "ritual" {
		dest = "ritual"
	}

	tmplRoot, err := templateRootFromCtx(ctx)
	if err != nil {
		return err
	}
	required := uniqueStrings(collectRequiredVars(ctx, tmplRoot, defaultFestivalTemplatePaths(tmplRoot)))
	vars := map[string]interface{}{
		"festival_name": name,
		"festival_goal": goal,
	}
	if tags != "" {
		vars["festival_tags"] = strings.Split(tags, ",")
	}
	for _, v := range required {
		if v == "festival_name" || v == "festival_goal" || v == "festival_tags" || v == "festival_description" {
			continue
		}
		if _, ok := vars[v]; ok {
			continue
		}
		val := strings.TrimSpace(display.PromptDefault(fmt.Sprintf("%s", v), ""))
		if val != "" {
			vars[v] = val
		}
	}

	varsFile, err := writeTempVarsFile(vars)
	if err != nil {
		return err
	}

	opts := &shared.CreateFestivalOpts{
		Name:     name,
		Goal:     goal,
		Tags:     tags,
		Type:     festType,
		VarsFile: varsFile,
		Project:  project,
		Seed:     seed,
		Dest:     dest,
	}
	return shared.RunCreateFestival(ctx, opts)
}

func festivalTypeSupportsSeedFallback(ft *types.FestivalType) bool {
	if ft == nil || ft.SkipIngestion {
		return false
	}
	for _, p := range ft.Phases {
		if p.Type == "ingest" || strings.EqualFold(p.Name, "INGEST") {
			return true
		}
	}
	return false
}

// Wizard: create festival (same path as quick create; dual plan path retired)
func tuiPlanFestivalWizard(ctx context.Context, display *ui.UI) error {
	return tuiCreateFestival(ctx, display)
}

func tuiGenerateFestivalGoal(ctx context.Context, display *ui.UI) error {
	if _, err := templateRootFromCtx(ctx); err != nil {
		return err
	}
	festDir := strings.TrimSpace(display.PromptDefault("Festival directory", "."))
	name := strings.TrimSpace(display.PromptDefault("festival_name", ""))
	goal := strings.TrimSpace(display.PromptDefault("festival_goal", ""))
	vars := map[string]interface{}{}
	if name != "" {
		vars["festival_name"] = name
	}
	if goal != "" {
		vars["festival_goal"] = goal
	}
	varsFile, err := writeTempVarsFile(vars)
	if err != nil {
		return err
	}
	destPath := filepath.Join(festDir, "FESTIVAL_GOAL.md")
	return shared.RunApply(ctx, &shared.ApplyOpts{TemplatePath: "FESTIVAL_GOAL_TEMPLATE.md", DestPath: destPath, VarsFile: varsFile})
}
