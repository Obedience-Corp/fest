package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
)

const standaloneStatusRenderComplete = "complete"

func runStandaloneStatus(ctx context.Context, jsonOutput bool) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, nil
	}
	info, err := show.ResolveStandaloneWorkflow(ctx, cwd)
	if err != nil {
		return true, err
	}
	if info == nil {
		return false, nil
	}
	if jsonOutput {
		out, err := renderStandaloneStatusJSON(info)
		if err != nil {
			return true, err
		}
		fmt.Println(out)
		return true, nil
	}
	fmt.Print(renderStandaloneStatusText(info))
	return true, nil
}

func collectStandaloneStatus(info *show.StandaloneWorkflowInfo) workflowStatusJSON {
	complete := info.RenderMode == standaloneStatusRenderComplete
	completed := info.CompletedSteps
	out := workflowStatusJSON{
		SchemaVersion:  workflowStatusSchema,
		Mode:           info.Mode,
		Workflow:       filepath.Base(info.StartDir),
		TotalSteps:     info.TotalSteps,
		Complete:       complete,
		Steps:          make([]workflowStatusStepJSON, 0, len(info.Steps)),
		WorkflowDoc:    info.WorkflowDoc,
		RuntimeDir:     info.RuntimeDir,
		RunID:          info.RunID,
		RunStatus:      info.RunStatus,
		CompletedSteps: &completed,
		Blocked:        info.Blocked,
		DocHashChanged: info.DocHashChanged,
	}
	if !complete {
		current := info.CurrentStep
		out.CurrentStep = &current
	}
	for _, step := range info.Steps {
		out.Steps = append(out.Steps, workflowStatusStepJSON{
			Number:        step.Number,
			Name:          step.Name,
			Status:        string(step.Status),
			IsCurrent:     step.IsCurrent,
			HasCheckpoint: step.HasCheckpoint,
			Goal:          step.Goal,
		})
		if step.IsCurrent {
			name := step.Name
			out.WorkflowStep = &name
		}
	}
	return out
}

func renderStandaloneStatusJSON(info *show.StandaloneWorkflowInfo) (string, error) {
	data, err := json.MarshalIndent(collectStandaloneStatus(info), "", "  ")
	if err != nil {
		return "", errors.Parse("formatting workflow status JSON", err)
	}
	return string(data), nil
}

func renderStandaloneStatusText(info *show.StandaloneWorkflowInfo) string {
	var sb strings.Builder
	sb.WriteString(ui.Category("Workflow Status"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "%s%s\n", ui.Label("Workflow: "), info.WorkflowDoc)
	fmt.Fprintf(&sb, "%s%s\n", ui.Label("Mode: "), strings.TrimPrefix(info.Mode, "standalone-"))
	if info.RunID != "" {
		fmt.Fprintf(&sb, "%s%s (%s)\n", ui.Label("Run: "), info.RunID, info.RunStatus)
	} else {
		fmt.Fprintf(&sb, "%s%s\n", ui.Label("Status: "), info.RunStatus)
	}
	if info.Blocked {
		fmt.Fprintf(&sb, "%s%s\n", ui.Label("Blocked: "), ui.Warning("true"))
	}
	sb.WriteString("\n")

	complete := info.RenderMode == standaloneStatusRenderComplete
	sb.WriteString(ui.Label("Current Step: "))
	if complete {
		sb.WriteString(ui.Success("Complete"))
	} else {
		fmt.Fprintf(&sb, "%d of %d", info.CurrentStep, info.TotalSteps)
	}
	sb.WriteString("\n\n")

	sb.WriteString(ui.Label("Steps:"))
	sb.WriteString("\n")
	sb.WriteString(shared.RenderWorkflowSteps(info.Steps, false))

	sb.WriteString("\n")
	sb.WriteString(ui.Label("Progress: "))
	sb.WriteString(shared.RenderWorkflowProgress(info.CompletedSteps, info.TotalSteps))
	sb.WriteString("\n")

	if info.DocHashChanged {
		fmt.Fprintf(&sb, "\n%s WORKFLOW.md has changed since this run started\n", ui.Warning("!"))
	}
	if complete {
		sb.WriteString("\n")
		sb.WriteString(ui.Success("✓ All steps complete"))
		sb.WriteString("\n")
	}
	return sb.String()
}
