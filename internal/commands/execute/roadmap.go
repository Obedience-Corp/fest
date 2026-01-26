// Package execute provides the fest execute command for festival orchestration.
package execute

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/orchestration"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// generateRoadmap creates a complete festival execution roadmap showing
// all phases, sequences, and tasks in a tree structure.
func generateRoadmap(ctx context.Context, festivalPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Build execution plan
	builder := orchestration.NewPlanBuilder(festivalPath)
	plan, err := builder.BuildPlan(ctx)
	if err != nil {
		return errors.Wrap(err, "building execution plan")
	}

	output := formatRoadmap(festivalPath, plan)
	fmt.Print(output)
	return nil
}

// formatRoadmap formats the execution plan as a readable roadmap.
func formatRoadmap(festivalPath string, plan *orchestration.ExecutionPlan) string {
	var buf strings.Builder

	// Festival header
	festivalName := filepath.Base(festivalPath)
	buf.WriteString(ui.H1(fmt.Sprintf("Festival Roadmap: %s", festivalName)))
	buf.WriteString("\n")
	buf.WriteString(ui.Dim(festivalPath))
	buf.WriteString("\n\n")

	// Summary
	if plan.Summary != nil {
		buf.WriteString(ui.H2("Summary"))
		buf.WriteString(fmt.Sprintf("  %s %d\n", ui.Label("Total Phases:"), len(plan.Phases)))
		buf.WriteString(fmt.Sprintf("  %s %d\n", ui.Label("Total Tasks:"), plan.Summary.TotalTasks))
		buf.WriteString("\n")
	}

	// Phases
	for _, phase := range plan.Phases {
		formatPhase(&buf, phase)
	}

	return buf.String()
}

// formatPhase formats a single phase for the roadmap.
func formatPhase(buf *strings.Builder, phase *orchestration.PhaseExecution) {
	// Phase header - use the name as-is since it already includes the number prefix
	buf.WriteString(ui.H2(fmt.Sprintf("Phase: %s", phase.Name)))
	buf.WriteString(fmt.Sprintf("  %s %d tasks\n", ui.Label("Tasks:"), phase.TotalTasks))

	// Detect and display mode for this phase
	phaseType, mode := detectPhaseMode(phase.Path)
	if phaseType != "" {
		buf.WriteString(fmt.Sprintf("  %s %s\n", ui.Label("Type:"), ui.Dim(phaseType)))
	}
	modeCmd := guidance.GetModeCommands(mode)
	buf.WriteString(fmt.Sprintf("  %s %s\n", ui.Label("Mode:"), ui.Value(string(mode))))
	buf.WriteString(fmt.Sprintf("  %s %s\n", ui.Label("Command:"), ui.Value(modeCmd.StartCommand)))
	buf.WriteString("\n")

	// Sequences
	for _, seq := range phase.Sequences {
		formatSequence(buf, seq)
	}

	// Quality gate if present
	if phase.QualityGate != nil {
		buf.WriteString(fmt.Sprintf("  %s %s\n", ui.Label("Quality Gate:"), ui.Value(phase.QualityGate.Type)))
	}

	buf.WriteString("\n")
}

// detectPhaseMode reads the phase type from PHASE_GOAL.md and returns the
// phase type string and corresponding guidance mode.
func detectPhaseMode(phasePath string) (string, guidance.Mode) {
	phaseGoalPath := filepath.Join(phasePath, "PHASE_GOAL.md")

	content, err := os.ReadFile(phaseGoalPath)
	if err != nil {
		// Default to implementation mode if no phase goal file
		return "", guidance.ModeImplementation
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		// Default to implementation mode if parse error or no frontmatter
		return "", guidance.ModeImplementation
	}

	// Get phase type from frontmatter
	phaseType := string(fm.PhaseType)
	if phaseType == "" {
		return "", guidance.ModeImplementation
	}

	mode := guidance.ModeFromPhaseType(phaseType)
	return phaseType, mode
}

// formatSequence formats a single sequence for the roadmap.
func formatSequence(buf *strings.Builder, seq *orchestration.SequenceExecution) {
	// Sequence header - use the name as-is since it already includes the number prefix
	buf.WriteString(fmt.Sprintf("  %s\n", ui.Value(seq.Name, ui.SequenceColor)))

	// Tasks
	for i, step := range seq.Steps {
		isLast := i == len(seq.Steps)-1
		formatStep(buf, step, isLast)
	}
}

// formatStep formats a step group (single task or parallel tasks) for the roadmap.
func formatStep(buf *strings.Builder, step *orchestration.StepGroup, isLast bool) {
	prefix := "├──"
	if isLast {
		prefix = "└──"
	}

	if len(step.Tasks) == 1 {
		// Single task
		task := step.Tasks[0]
		taskName := filepath.Base(task.Path)
		buf.WriteString(fmt.Sprintf("    %s %s\n", prefix, ui.Dim(taskName)))
	} else {
		// Parallel task group
		buf.WriteString(fmt.Sprintf("    %s %s\n", prefix, ui.Info("[Parallel]")))
		for j, task := range step.Tasks {
			taskName := filepath.Base(task.Path)
			innerPrefix := "│   ├──"
			if j == len(step.Tasks)-1 {
				if isLast {
					innerPrefix = "    └──"
				} else {
					innerPrefix = "│   └──"
				}
			}
			buf.WriteString(fmt.Sprintf("    %s %s\n", innerPrefix, ui.Dim(taskName)))
		}
	}
}
