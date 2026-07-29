//go:build !no_charm

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/types"
	"github.com/Obedience-Corp/fest/internal/workspace"
	uitheme "github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/charmbracelet/huh"
)

// charmCreateFestival runs the multi-step human Festival create wizard.
// Type list comes from LoadFestivalTypesConfig (not a hardcoded slice).
func charmCreateFestival(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg, err := types.LoadFestivalTypesConfig(ctx)
	if err != nil {
		return err
	}
	if len(cfg.Types) == 0 {
		return fmt.Errorf("no festival types configured")
	}

	projects := listCampaignProjects(ctx)

	draft := festivalDraft{TypeName: defaultTypeName(cfg)}
	step := 1
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch step {
		case 1:
			ok, err := stepFestivalType(ctx, cfg, &draft)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			step = 2
		case 2:
			ok, err := stepFestivalIdentity(ctx, cfg, &draft)
			if err != nil {
				return err
			}
			if !ok {
				step = 1
				continue
			}
			step = 3
		case 3:
			ok, err := stepFestivalProject(ctx, projects, &draft)
			if err != nil {
				return err
			}
			if !ok {
				step = 2
				continue
			}
			ft := typeByName(cfg, draft.TypeName)
			if festivalTypeSupportsSeed(ft) {
				step = 4
			} else {
				draft.Seed = ""
				step = 5
			}
		case 4:
			ok, err := stepFestivalSeed(ctx, &draft)
			if err != nil {
				return err
			}
			if !ok {
				step = 3
				continue
			}
			step = 5
		case 5:
			ok, err := stepFestivalDetails(ctx, &draft)
			if err != nil {
				return err
			}
			if !ok {
				ft := typeByName(cfg, draft.TypeName)
				if festivalTypeSupportsSeed(ft) {
					step = 4
				} else {
					step = 3
				}
				continue
			}
			step = 6
		case 6:
			action, err := stepFestivalConfirm(ctx, cfg, &draft)
			if err != nil {
				return err
			}
			switch action {
			case "create":
				return submitFestivalCreate(ctx, cfg, &draft)
			case "back":
				step = 5
			default:
				return nil
			}
		default:
			return fmt.Errorf("unknown festival create step %d", step)
		}
	}
}

type festivalDraft struct {
	TypeName string
	Name     string
	Goal     string
	Project  string
	Seed     string
	Tags     string
}

func defaultTypeName(cfg *types.FestivalTypesConfig) string {
	if d := cfg.GetDefaultType(); d != nil {
		return d.Name
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

func festivalTypeSupportsSeed(ft *types.FestivalType) bool {
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

func festivalTypeOptionLabel(ft types.FestivalType) string {
	auto := ft.GetAutoPhases()
	autoStr := "none"
	if len(auto) > 0 {
		names := make([]string, len(auto))
		for i, p := range auto {
			names[i] = p.Name
		}
		autoStr = strings.Join(names, "→")
	}
	name := ft.Name
	if ft.Default {
		name += "*"
	}
	desc := ft.Description
	if len(desc) > 42 {
		desc = desc[:39] + "…"
	}
	return fmt.Sprintf("%-15s %-42s  %s  [%s/]", name, desc, autoStr, festivalTypeDest(&ft))
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

func stepFestivalType(ctx context.Context, cfg *types.FestivalTypesConfig, d *festivalDraft) (bool, error) {
	opts := make([]huh.Option[string], 0, len(cfg.Types))
	for _, t := range cfg.Types {
		opts = append(opts, huh.NewOption(festivalTypeOptionLabel(t), t.Name))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Type").
				Description("What kind of festival is this?\nTypes load from festival_types.yaml."),
			huh.NewSelect[string]().
				Title("Festival type").
				Description("j/k or ↑/↓ · enter next · esc cancel").
				Options(opts...).
				Value(&d.TypeName),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	return d.TypeName != "", nil
}

func stepFestivalIdentity(ctx context.Context, cfg *types.FestivalTypesConfig, d *festivalDraft) (bool, error) {
	ft := typeByName(cfg, d.TypeName)
	ctxLine := fmt.Sprintf("Type: %s · Auto: %s · Dest: festivals/%s/",
		ft.Name, autoPhaseSummary(ft), festivalTypeDest(ft))
	name, goal := d.Name, d.Goal
	cont := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Identity").
				Description(ctxLine),
			huh.NewInput().
				Title("Name *").
				Placeholder("e.g., ecommerce-mvp").
				Value(&name).
				Validate(notEmpty),
			huh.NewText().
				Title("Goal").
				Placeholder("What does done look like?").
				Description("Encouraged — agents plan better with a goal").
				CharLimit(5000).
				Lines(3).
				Value(&goal),
			huh.NewConfirm().
				Title("Continue to project step?").
				Affirmative("Next").
				Negative("Back").
				Value(&cont),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	d.Name = strings.TrimSpace(name)
	d.Goal = strings.TrimSpace(goal)
	return cont, nil
}

func stepFestivalProject(ctx context.Context, projects []string, d *festivalDraft) (bool, error) {
	const (
		pick = "pick"
		path = "path"
		skip = "skip"
	)
	choice := skip
	if d.Project != "" {
		choice = pick
	}
	cont := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Project").
				Description("Link a project this festival will own?\nRecommended for implementation work."),
			huh.NewSelect[string]().
				Title("Project").
				Options(
					huh.NewOption("Pick from projects list", pick),
					huh.NewOption("Enter a custom path", path),
					huh.NewOption("Skip for now", skip),
				).
				Value(&choice),
			huh.NewConfirm().
				Title("Continue?").
				Affirmative("Next").
				Negative("Back").
				Value(&cont),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	if !cont {
		return false, nil
	}

	switch choice {
	case skip:
		d.Project = ""
		return true, nil
	case path:
		p := d.Project
		next := true
		pf := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Project path").
					Placeholder("projects/my-app").
					Value(&p),
				huh.NewConfirm().
					Title("Continue?").
					Affirmative("Next").
					Negative("Back").
					Value(&next),
			),
		)
		if err := uitheme.RunForm(ctx, pf); err != nil {
			if uitheme.IsCancelled(err) {
				return false, nil
			}
			return false, err
		}
		if !next {
			return stepFestivalProject(ctx, projects, d)
		}
		d.Project = strings.TrimSpace(p)
		return true, nil
	default:
		if len(projects) == 0 {
			// No projects found — fall back to custom path entry.
			p := d.Project
			next := true
			pf := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().Title("No projects/* found").Description("Enter a path or go back and skip."),
					huh.NewInput().Title("Project path").Placeholder("projects/my-app").Value(&p),
					huh.NewConfirm().Title("Continue?").Affirmative("Next").Negative("Back").Value(&next),
				),
			)
			if err := uitheme.RunForm(ctx, pf); err != nil {
				if uitheme.IsCancelled(err) {
					return false, nil
				}
				return false, err
			}
			if !next {
				return stepFestivalProject(ctx, projects, d)
			}
			d.Project = strings.TrimSpace(p)
			return true, nil
		}
		picked := d.Project
		if picked == "" {
			picked = projects[0]
		}
		popts := make([]huh.Option[string], 0, len(projects))
		for _, p := range projects {
			popts = append(popts, huh.NewOption(p, p))
		}
		next := true
		pf := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Choose project").
					Options(popts...).
					Value(&picked),
				huh.NewConfirm().
					Title("Continue?").
					Affirmative("Next").
					Negative("Back").
					Value(&next),
			),
		)
		if err := uitheme.RunForm(ctx, pf); err != nil {
			if uitheme.IsCancelled(err) {
				return false, nil
			}
			return false, err
		}
		if !next {
			return stepFestivalProject(ctx, projects, d)
		}
		d.Project = picked
		return true, nil
	}
}

func stepFestivalSeed(ctx context.Context, d *festivalDraft) (bool, error) {
	const (
		paste = "paste"
		skip  = "skip"
	)
	choice := skip
	if strings.TrimSpace(d.Seed) != "" {
		choice = paste
	}
	cont := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Seed").
				Description("Starting material for INGEST?\nOptional — agents can gather context during the phase."),
			huh.NewSelect[string]().
				Title("Seed").
				Options(
					huh.NewOption("Paste a short brief", paste),
					huh.NewOption("Skip — agent will gather during INGEST", skip),
				).
				Value(&choice),
			huh.NewConfirm().
				Title("Continue?").
				Affirmative("Next").
				Negative("Back").
				Value(&cont),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	if !cont {
		return false, nil
	}
	if choice == skip {
		d.Seed = ""
		return true, nil
	}
	seed := d.Seed
	next := true
	sf := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Brief").
				Placeholder("Prior art, links, constraints…").
				CharLimit(8000).
				Lines(5).
				Value(&seed),
			huh.NewConfirm().
				Title("Continue?").
				Affirmative("Next").
				Negative("Back").
				Value(&next),
		),
	)
	if err := uitheme.RunForm(ctx, sf); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	if !next {
		return stepFestivalSeed(ctx, d)
	}
	d.Seed = strings.TrimSpace(seed)
	return true, nil
}

func stepFestivalDetails(ctx context.Context, d *festivalDraft) (bool, error) {
	tags := d.Tags
	cont := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Details").
				Description("Optional tags for filtering and grouping."),
			huh.NewInput().
				Title("Tags (comma-separated)").
				Placeholder("backend,commerce").
				Value(&tags),
			huh.NewConfirm().
				Title("Continue to confirm?").
				Affirmative("Next").
				Negative("Back").
				Value(&cont),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return false, nil
		}
		return false, err
	}
	d.Tags = strings.TrimSpace(tags)
	return cont, nil
}

func stepFestivalConfirm(ctx context.Context, cfg *types.FestivalTypesConfig, d *festivalDraft) (string, error) {
	summary := buildFestivalConfirmSummary(cfg, d)
	action := "create"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("New Festival · Confirm").
				Description(summary),
			huh.NewSelect[string]().
				Title("Action").
				Options(
					huh.NewOption("Create festival", "create"),
					huh.NewOption("Back", "back"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&action),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return "cancel", nil
		}
		return "cancel", err
	}
	return action, nil
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
	if len(goal) > 80 {
		goal = goal[:79] + "…"
	}
	return fmt.Sprintf(
		"Name:     %s\nType:     %s\nDest:     festivals/%s/\nPhases:   %s\nProject:  %s\nSeed:     %s\nTags:     %s\nGoal:     %s",
		d.Name, ft.Name, festivalTypeDest(ft), autoStr, proj, seed, tags, goal,
	)
}

func submitFestivalCreate(ctx context.Context, cfg *types.FestivalTypesConfig, d *festivalDraft) error {
	ft := typeByName(cfg, d.TypeName)
	dest := festivalTypeDest(ft)

	vars := map[string]interface{}{"festival_name": d.Name, "festival_goal": d.Goal}
	if strings.TrimSpace(d.Tags) != "" {
		vars["festival_tags"] = strings.Split(d.Tags, ",")
	}
	varsFile, err := writeTempVarsFile(vars)
	if err != nil {
		return err
	}

	opts := &shared.CreateFestivalOpts{
		Name:     d.Name,
		Goal:     d.Goal,
		Tags:     d.Tags,
		Type:     d.TypeName,
		VarsFile: varsFile,
		Project:  d.Project,
		Seed:     d.Seed,
		Dest:     dest,
	}
	return shared.RunCreateFestival(ctx, opts)
}

// listCampaignProjects returns relative projects/* paths under the campaign root.
func listCampaignProjects(ctx context.Context) []string {
	root, err := workspace.DetectCampaign(ctx, "")
	if err != nil || root == "" {
		return nil
	}
	projectsDir := filepath.Join(root, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "worktrees" {
			continue
		}
		out = append(out, filepath.Join("projects", name))
	}
	return out
}

// charmPlanFestivalWizard is retained for any legacy callers but now routes to the
// single Festival wizard (plan-vs-quick dual is retired).
func charmPlanFestivalWizard(ctx context.Context) error {
	return charmCreateFestival(ctx)
}

func charmGenerateFestivalGoal(ctx context.Context) error {
	if _, err := templateRootFromCtx(ctx); err != nil {
		return err
	}
	var festDir, name, goal string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Festival directory").Placeholder(".").Value(&festDir),
			huh.NewInput().Title("festival_name").Value(&name),
			huh.NewInput().Title("festival_goal").Value(&goal),
		),
	)
	if err := uitheme.RunForm(ctx, form); err != nil {
		if uitheme.IsCancelled(err) {
			return nil
		}
		return err
	}
	vars := map[string]interface{}{}
	if strings.TrimSpace(name) != "" {
		vars["festival_name"] = name
	}
	if strings.TrimSpace(goal) != "" {
		vars["festival_goal"] = goal
	}
	varsFile, err := writeTempVarsFile(vars)
	if err != nil {
		return err
	}
	destPath := filepath.Join(festDir, "FESTIVAL_GOAL.md")
	return shared.RunApply(ctx, &shared.ApplyOpts{TemplatePath: "FESTIVAL_GOAL_TEMPLATE.md", DestPath: destPath, VarsFile: varsFile})
}
