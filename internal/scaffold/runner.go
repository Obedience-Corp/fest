package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	tpl "github.com/Obedience-Corp/fest/internal/template"
)

const (
	dirPerms  = 0755
	filePerms = 0644
)

// RunnerOptions configures the scaffold runner.
type RunnerOptions struct {
	// FestivalDir is the destination directory for the festival.
	FestivalDir string

	// TemplateRoot is the path to .festival/templates/.
	// If empty, the runner falls back to generating minimal content inline.
	TemplateRoot string

	// DryRun previews without creating files.
	DryRun bool

	// SkipMarkers skips marker processing.
	SkipMarkers bool
}

// RunResult contains the outcome of a scaffold run.
type RunResult struct {
	// FestivalDir is the path to the created festival.
	FestivalDir string

	// PhasesCreated is the number of phases created.
	PhasesCreated int

	// SequencesCreated is the number of sequences created.
	SequencesCreated int

	// TasksCreated is the number of tasks created.
	TasksCreated int

	// FilesCreated lists all created file paths.
	FilesCreated []string

	// DirsCreated lists all created directory paths.
	DirsCreated []string
}

// Runner generates a festival structure from a parsed plan.
type Runner struct {
	opts RunnerOptions
}

// NewRunner creates a new scaffold runner.
func NewRunner(opts RunnerOptions) *Runner {
	return &Runner{opts: opts}
}

// Run generates the festival structure from a parsed plan.
func (r *Runner) Run(ctx context.Context, plan *ParsedPlan) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := &RunResult{
		FestivalDir: r.opts.FestivalDir,
	}

	// Create festival root directory
	if !r.opts.DryRun {
		if err := os.MkdirAll(r.opts.FestivalDir, dirPerms); err != nil {
			return nil, fmt.Errorf("creating festival directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, r.opts.FestivalDir)

	// Build festival-level template context
	festCtx := tpl.NewContext()
	festCtx.SetFestival(plan.FestivalName, plan.Goal, nil)
	festCtx.ComputeStructureVariables()

	// Create FESTIVAL_GOAL.md
	if err := r.createFestivalGoal(ctx, plan, festCtx, result); err != nil {
		return nil, err
	}

	// Create FESTIVAL_OVERVIEW.md
	if err := r.createFestivalOverview(ctx, plan, festCtx, result); err != nil {
		return nil, err
	}

	// Create phases
	for _, phase := range plan.Phases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := r.createPhase(ctx, plan, &phase, festCtx, result); err != nil {
			return nil, fmt.Errorf("creating phase %s: %w", phase.Name, err)
		}
	}

	return result, nil
}

// renderTemplate loads a template from TemplateRoot, renders Go template variables
// and auto-fills [REPLACE: ...] markers. Returns empty string and nil error if the
// template file doesn't exist (caller should fall back to inline content).
func (r *Runner) renderTemplate(ctx context.Context, relPath string, tmplCtx *tpl.Context) (string, bool, error) {
	if r.opts.TemplateRoot == "" {
		return "", false, nil
	}

	tpath := filepath.Join(r.opts.TemplateRoot, relPath)
	if _, err := os.Stat(tpath); os.IsNotExist(err) {
		return "", false, nil
	}

	loader := tpl.NewLoader()
	t, err := loader.Load(ctx, tpath)
	if err != nil {
		return "", false, fmt.Errorf("loading template %s: %w", relPath, err)
	}

	mgr := tpl.NewManager()
	requires := t.Metadata != nil && len(t.Metadata.RequiredVariables) > 0
	var content string
	if requires || strings.Contains(t.Content, "{{") {
		content, err = mgr.Render(t, tmplCtx)
		if err != nil {
			return "", false, fmt.Errorf("rendering template %s: %w", relPath, err)
		}
	} else {
		content = t.Content
	}

	// Auto-fill [REPLACE: ...] markers from context
	if !r.opts.SkipMarkers {
		renderer := tpl.NewRenderer()
		if rendered, renderErr := renderer.RenderWithMarkerReplacement(content, tmplCtx, nil); renderErr == nil {
			content = rendered
		}
	}

	// Strip template metadata frontmatter (we inject our own)
	content = stripTemplateFrontmatter(content)

	return content, true, nil
}

// stripTemplateFrontmatter removes YAML frontmatter from template content.
// The scaffold injects its own frontmatter via the frontmatter package.
func stripTemplateFrontmatter(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return content
	}
	rest := trimmed[3:]
	endIdx := strings.Index(rest, "---")
	if endIdx == -1 {
		return content
	}
	after := rest[endIdx+3:]
	return strings.TrimLeft(after, "\n\r")
}

// createFestivalGoal generates the FESTIVAL_GOAL.md file.
func (r *Runner) createFestivalGoal(ctx context.Context, plan *ParsedPlan, festCtx *tpl.Context, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Try template-based rendering first
	content, ok, err := r.renderTemplate(ctx, "festival/GOAL.md", festCtx)
	if err != nil {
		return err
	}

	// Fallback to inline content
	if !ok {
		goal := plan.Goal
		if goal == "" {
			goal = fmt.Sprintf("Festival: %s", plan.FestivalName)
		}
		content = fmt.Sprintf("# Festival Goal\n\n%s\n", goal)
	}

	fm := frontmatter.NewFrontmatter(frontmatter.TypeFestival, plan.FestivalName, plan.FestivalName)
	fm.Status = frontmatter.StatusPlanning

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting festival frontmatter: %w", err)
	}

	path := filepath.Join(r.opts.FestivalDir, "FESTIVAL_GOAL.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(path, []byte(fullContent), filePerms); err != nil {
			return fmt.Errorf("writing FESTIVAL_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, path)
	return nil
}

// createFestivalOverview generates the FESTIVAL_OVERVIEW.md file.
func (r *Runner) createFestivalOverview(ctx context.Context, plan *ParsedPlan, festCtx *tpl.Context, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Try template-based rendering first
	content, ok, err := r.renderTemplate(ctx, "festival/OVERVIEW.md", festCtx)
	if err != nil {
		return err
	}

	// Fallback to inline content with plan data
	if !ok {
		content = buildOverviewContent(plan)
	}

	fm := frontmatter.NewFrontmatter(frontmatter.TypeFestival, plan.FestivalName, plan.FestivalName)
	fm.Status = frontmatter.StatusPlanning

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting overview frontmatter: %w", err)
	}

	path := filepath.Join(r.opts.FestivalDir, "FESTIVAL_OVERVIEW.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(path, []byte(fullContent), filePerms); err != nil {
			return fmt.Errorf("writing FESTIVAL_OVERVIEW.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, path)
	return nil
}

// buildOverviewContent generates fallback FESTIVAL_OVERVIEW.md content from the plan.
func buildOverviewContent(plan *ParsedPlan) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Festival Overview: %s\n\n", plan.FestivalName))
	b.WriteString("## Problem Statement\n\n")
	if plan.Goal != "" {
		b.WriteString(fmt.Sprintf("**Desired State:** %s\n\n", plan.Goal))
	} else {
		b.WriteString("**Desired State:** [REPLACE: What we want to achieve]\n\n")
	}

	b.WriteString("## Scope\n\n### In Scope\n\n")
	if len(plan.Phases) > 0 {
		for _, p := range plan.Phases {
			b.WriteString(fmt.Sprintf("- %s (%s)\n", p.Name, p.Type))
		}
	} else {
		b.WriteString("- [REPLACE: What's included in this festival]\n")
	}

	b.WriteString("\n## Planned Phases\n\n")
	for _, p := range plan.Phases {
		b.WriteString(fmt.Sprintf("### %s (%s)\n\n", p.Name, p.Type))
		if p.Description != "" {
			b.WriteString(p.Description + "\n\n")
		}
		if len(p.Sequences) > 0 {
			b.WriteString(fmt.Sprintf("Sequences: %d", len(p.Sequences)))
			taskCount := 0
			for _, s := range p.Sequences {
				taskCount += len(s.Tasks)
			}
			if taskCount > 0 {
				b.WriteString(fmt.Sprintf(", Tasks: %d", taskCount))
			}
			b.WriteString("\n\n")
		}
	}

	b.WriteString("## Notes\n\n")
	b.WriteString("[REPLACE: Any additional context, assumptions, or open questions]\n")

	return b.String()
}

// createPhase generates a phase directory and PHASE_GOAL.md.
func (r *Runner) createPhase(ctx context.Context, plan *ParsedPlan, phase *ParsedPhase, _ *tpl.Context, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	phaseID := tpl.FormatPhaseID(phase.Number, phase.Name)
	phaseDir := filepath.Join(r.opts.FestivalDir, phaseID)

	if !r.opts.DryRun {
		if err := os.MkdirAll(phaseDir, dirPerms); err != nil {
			return fmt.Errorf("creating phase directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, phaseDir)
	result.PhasesCreated++

	// Build phase-level context (inherits festival context)
	phaseCtx := tpl.NewContext()
	phaseCtx.SetFestival(plan.FestivalName, plan.Goal, nil)
	phaseCtx.SetPhase(phase.Number, phase.Name, phase.Type)
	if phase.Description != "" {
		phaseCtx.SetPhaseObjective(phase.Description)
	}
	phaseCtx.ComputeStructureVariables()

	// Try template-based rendering: phases/{type}/GOAL.md
	pt := strings.ToLower(phase.Type)
	templatePath := filepath.Join("phases", pt, "GOAL.md")
	content, ok, err := r.renderTemplate(ctx, templatePath, phaseCtx)
	if err != nil {
		return err
	}

	// Fallback to inline content
	if !ok {
		description := phase.Description
		if description == "" {
			description = fmt.Sprintf("Phase %d: %s", phase.Number, phase.Name)
		}
		content = fmt.Sprintf("# Phase Goal\n\n%s\n", description)
	}

	fm := frontmatter.NewPhaseFrontmatter(
		phaseID,
		strings.ToLower(phase.Name),
		plan.FestivalName,
		phase.Number,
		frontmatter.PhaseType(phase.Type),
	)

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting phase frontmatter: %w", err)
	}

	goalPath := filepath.Join(phaseDir, "PHASE_GOAL.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(goalPath, []byte(fullContent), filePerms); err != nil {
			return fmt.Errorf("writing PHASE_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, goalPath)

	// Create sequences
	for _, seq := range phase.Sequences {
		if err := r.createSequence(ctx, phaseDir, phaseID, plan, phase, &seq, phaseCtx, result); err != nil {
			return fmt.Errorf("creating sequence %s: %w", seq.Name, err)
		}
	}

	return nil
}

// createSequence generates a sequence directory with SEQUENCE_GOAL.md and task files.
func (r *Runner) createSequence(ctx context.Context, phaseDir, phaseID string, plan *ParsedPlan, phase *ParsedPhase, seq *ParsedSequence, _ *tpl.Context, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	seqID := tpl.FormatSequenceID(seq.Number, seq.Name)
	seqDir := filepath.Join(phaseDir, seqID)

	if !r.opts.DryRun {
		if err := os.MkdirAll(seqDir, dirPerms); err != nil {
			return fmt.Errorf("creating sequence directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, seqDir)
	result.SequencesCreated++

	// Build sequence-level context
	seqCtx := tpl.NewContext()
	seqCtx.SetFestival(plan.FestivalName, plan.Goal, nil)
	seqCtx.SetPhase(phase.Number, phase.Name, phase.Type)
	seqCtx.SetSequence(seq.Number, seq.Name)
	if seq.Requirement != "" {
		seqCtx.SetSequenceObjective(seq.Requirement)
	}
	seqCtx.ComputeStructureVariables()

	// Try template-based rendering
	content, ok, err := r.renderTemplate(ctx, "sequences/GOAL.md", seqCtx)
	if err != nil {
		return err
	}

	// Fallback to inline content
	if !ok {
		goalText := fmt.Sprintf("Sequence %d: %s", seq.Number, seq.Name)
		if seq.Requirement != "" {
			goalText += fmt.Sprintf(" (%s)", seq.Requirement)
		}
		content = fmt.Sprintf("# Sequence Goal\n\n%s\n", goalText)
	}

	fm := frontmatter.NewSequenceFrontmatter(seqID, seq.Name, phaseID, seq.Number)

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting sequence frontmatter: %w", err)
	}

	goalPath := filepath.Join(seqDir, "SEQUENCE_GOAL.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(goalPath, []byte(fullContent), filePerms); err != nil {
			return fmt.Errorf("writing SEQUENCE_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, goalPath)

	// Create task files
	for _, task := range seq.Tasks {
		if err := r.createTask(ctx, seqDir, seqID, plan, phase, seq, &task, result); err != nil {
			return fmt.Errorf("creating task %s: %w", task.Name, err)
		}
	}

	return nil
}

// createTask generates a task file with frontmatter.
func (r *Runner) createTask(ctx context.Context, seqDir, seqID string, plan *ParsedPlan, phase *ParsedPhase, seq *ParsedSequence, task *ParsedTask, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	taskID := tpl.FormatTaskID(task.Number, task.Name)
	taskPath := filepath.Join(seqDir, taskID)

	// Build task-level context
	taskCtx := tpl.NewContext()
	taskCtx.SetFestival(plan.FestivalName, plan.Goal, nil)
	taskCtx.SetPhase(phase.Number, phase.Name, phase.Type)
	taskCtx.SetSequence(seq.Number, seq.Name)
	taskCtx.SetTask(task.Number, task.Name)
	taskCtx.ComputeStructureVariables()

	// Try template-based rendering
	content, ok, err := r.renderTemplate(ctx, "tasks/TASK.md", taskCtx)
	if err != nil {
		return err
	}

	// Fallback to inline content
	if !ok {
		content = fmt.Sprintf("# Task: %s\n\n## Objective\n\n%s\n\n## Requirements\n\n- [ ] Implementation complete\n- [ ] Tests pass\n\n## Done When\n\n- [ ] All requirements met\n- [ ] `just build` passes\n",
			task.Name, task.Name)
	}

	fm := frontmatter.NewTaskFrontmatter(taskID, task.Name, seqID, task.Number, frontmatter.AutonomyMedium)

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting task frontmatter: %w", err)
	}

	if !r.opts.DryRun {
		if err := os.WriteFile(taskPath, []byte(fullContent), filePerms); err != nil {
			return fmt.Errorf("writing task file: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, taskPath)
	result.TasksCreated++

	return nil
}
