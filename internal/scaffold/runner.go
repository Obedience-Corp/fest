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

// RunnerOptions configures the scaffold runner.
type RunnerOptions struct {
	// FestivalDir is the destination directory for the festival.
	FestivalDir string

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
		if err := os.MkdirAll(r.opts.FestivalDir, 0755); err != nil {
			return nil, fmt.Errorf("creating festival directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, r.opts.FestivalDir)

	// Create FESTIVAL_GOAL.md
	if err := r.createFestivalGoal(ctx, plan, result); err != nil {
		return nil, err
	}

	// Create phases
	for _, phase := range plan.Phases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := r.createPhase(ctx, plan, &phase, result); err != nil {
			return nil, fmt.Errorf("creating phase %s: %w", phase.Name, err)
		}
	}

	return result, nil
}

// createFestivalGoal generates the FESTIVAL_GOAL.md file.
func (r *Runner) createFestivalGoal(ctx context.Context, plan *ParsedPlan, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	goal := plan.Goal
	if goal == "" {
		goal = fmt.Sprintf("Festival: %s", plan.FestivalName)
	}

	content := fmt.Sprintf("# Festival Goal\n\n%s\n", goal)

	fm := frontmatter.NewFrontmatter(frontmatter.TypeFestival, plan.FestivalName, plan.FestivalName)
	fm.Status = frontmatter.StatusPlanned

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting festival frontmatter: %w", err)
	}

	path := filepath.Join(r.opts.FestivalDir, "FESTIVAL_GOAL.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(path, []byte(fullContent), 0644); err != nil {
			return fmt.Errorf("writing FESTIVAL_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, path)
	return nil
}

// createPhase generates a phase directory and PHASE_GOAL.md.
func (r *Runner) createPhase(ctx context.Context, plan *ParsedPlan, phase *ParsedPhase, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	phaseID := tpl.FormatPhaseID(phase.Number, phase.Name)
	phaseDir := filepath.Join(r.opts.FestivalDir, phaseID)

	if !r.opts.DryRun {
		if err := os.MkdirAll(phaseDir, 0755); err != nil {
			return fmt.Errorf("creating phase directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, phaseDir)
	result.PhasesCreated++

	// Create PHASE_GOAL.md
	description := phase.Description
	if description == "" {
		description = fmt.Sprintf("Phase %d: %s", phase.Number, phase.Name)
	}

	content := fmt.Sprintf("# Phase Goal\n\n%s\n", description)

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
		if err := os.WriteFile(goalPath, []byte(fullContent), 0644); err != nil {
			return fmt.Errorf("writing PHASE_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, goalPath)

	// Create sequences
	for _, seq := range phase.Sequences {
		if err := r.createSequence(ctx, phaseDir, phaseID, &seq, result); err != nil {
			return fmt.Errorf("creating sequence %s: %w", seq.Name, err)
		}
	}

	return nil
}

// createSequence generates a sequence directory with SEQUENCE_GOAL.md and task files.
func (r *Runner) createSequence(ctx context.Context, phaseDir, phaseID string, seq *ParsedSequence, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	seqID := tpl.FormatSequenceID(seq.Number, seq.Name)
	seqDir := filepath.Join(phaseDir, seqID)

	if !r.opts.DryRun {
		if err := os.MkdirAll(seqDir, 0755); err != nil {
			return fmt.Errorf("creating sequence directory: %w", err)
		}
	}
	result.DirsCreated = append(result.DirsCreated, seqDir)
	result.SequencesCreated++

	// Create SEQUENCE_GOAL.md
	goalText := fmt.Sprintf("Sequence %d: %s", seq.Number, seq.Name)
	if seq.Requirement != "" {
		goalText += fmt.Sprintf(" (%s)", seq.Requirement)
	}

	content := fmt.Sprintf("# Sequence Goal\n\n%s\n", goalText)

	fm := frontmatter.NewSequenceFrontmatter(seqID, seq.Name, phaseID, seq.Number)

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting sequence frontmatter: %w", err)
	}

	goalPath := filepath.Join(seqDir, "SEQUENCE_GOAL.md")
	if !r.opts.DryRun {
		if err := os.WriteFile(goalPath, []byte(fullContent), 0644); err != nil {
			return fmt.Errorf("writing SEQUENCE_GOAL.md: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, goalPath)

	// Create task files
	for _, task := range seq.Tasks {
		if err := r.createTask(ctx, seqDir, seqID, &task, result); err != nil {
			return fmt.Errorf("creating task %s: %w", task.Name, err)
		}
	}

	return nil
}

// createTask generates a task file with frontmatter.
func (r *Runner) createTask(ctx context.Context, seqDir, seqID string, task *ParsedTask, result *RunResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	taskID := tpl.FormatTaskID(task.Number, task.Name)
	taskPath := filepath.Join(seqDir, taskID)

	content := fmt.Sprintf("# Task: %s\n\n## Objective\n\n%s\n\n## Requirements\n\n- [ ] Implementation complete\n- [ ] Tests pass\n\n## Done When\n\n- [ ] All requirements met\n- [ ] `just build` passes\n",
		task.Name, task.Name)

	fm := frontmatter.NewTaskFrontmatter(taskID, task.Name, seqID, task.Number, frontmatter.AutonomyMedium)

	fullContent, err := frontmatter.InjectString(content, fm)
	if err != nil {
		return fmt.Errorf("injecting task frontmatter: %w", err)
	}

	if !r.opts.DryRun {
		if err := os.WriteFile(taskPath, []byte(fullContent), 0644); err != nil {
			return fmt.Errorf("writing task file: %w", err)
		}
	}
	result.FilesCreated = append(result.FilesCreated, taskPath)
	result.TasksCreated++

	return nil
}
