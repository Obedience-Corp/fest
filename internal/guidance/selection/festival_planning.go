package selection

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/fest/embedded/templates/agent"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// FestivalPlanningResult describes a festival that has no executable work yet
// because its own plan is still unwritten: it is in planning status and has no
// phase to walk, or no actionable task or workflow step inside one.
//
// It is the payload behind the NextTaskResult.FestivalPlanning field, and its
// presence is what tells a consumer that fest next returned a planning step
// rather than a task. Task stays nil and FestivalComplete stays false.
type FestivalPlanningResult struct {
	// Status is the festival's lifecycle status, always "planning" today.
	Status string `json:"status"`

	// PhaseCount is the number of numbered phase directories in the festival.
	PhaseCount int `json:"phase_count"`

	// MarkerTotal is the number of unfilled template markers across all files.
	MarkerTotal int `json:"marker_total"`

	// MarkerFiles lists each file that still holds markers, with its count.
	MarkerFiles []PlanningMarkerFile `json:"marker_files,omitempty"`

	// NextActions lists the commands that move this festival forward, in order.
	NextActions []string `json:"next_actions,omitempty"`
}

// PlanningMarkerFile is one file with unfilled template markers.
type PlanningMarkerFile struct {
	// File is the path relative to the festival root.
	File string `json:"file"`
	// Count is the number of unfilled markers in that file.
	Count int `json:"count"`
}

// formatTextFestivalPlanning renders the planning step for a festival that has
// no executable work yet. Verbose output uses the same renderer: a planning
// step carries no task content for a verbose mode to expand.
func formatTextFestivalPlanning(result *NextTaskResult) string {
	p := result.FestivalPlanning

	var info strings.Builder
	if result.Location != nil && result.Location.FestivalPath != "" {
		ui.WriteLabelValue(&info, "Festival", ui.Dim(result.Location.FestivalPath))
	}
	ui.WriteLabelValue(&info, "Status", ui.Value(p.Status))
	ui.WriteLabelValue(&info, "Phases", ui.Value(fmt.Sprintf("%d", p.PhaseCount)))
	if result.Reason != "" {
		ui.WriteLabelValue(&info, "Reason", ui.Info(result.Reason))
	}

	var markers strings.Builder
	if len(p.MarkerFiles) > 0 {
		markers.WriteString(ui.H2("Unfilled Markers"))
		markers.WriteString("\n")
		for _, f := range p.MarkerFiles {
			fmt.Fprintf(&markers, "  %s %s %s\n",
				ui.StateIcon("pending"), f.File, ui.Dim(fmt.Sprintf("(%d)", f.Count)))
		}
		fmt.Fprintf(&markers, "\n  %s\n",
			ui.Info(fmt.Sprintf("%d markers in %d files", p.MarkerTotal, len(p.MarkerFiles))))
	}

	var steps strings.Builder
	steps.WriteString(ui.H2("Next Step"))
	steps.WriteString("\n")
	steps.WriteString("Write the plan. This festival has nothing to execute yet.\n\n")
	if len(p.MarkerFiles) > 0 {
		fmt.Fprintf(&steps, "  1. Fill the markers: %s\n", ui.Value("fest wizard fill ."))
		steps.WriteString("     Or edit the files directly.\n")
		fmt.Fprintf(&steps, "     Agents can scaffold them filled: %s\n",
			ui.Dim("fest create festival --agent --markers"))
	} else {
		fmt.Fprintf(&steps, "  1. Review the festival documents: %s\n", ui.Value("fest show"))
	}
	fmt.Fprintf(&steps, "  2. Add phases: %s\n", ui.Value("fest create phase --name PHASE_NAME"))
	fmt.Fprintf(&steps, "  3. Check the structure: %s\n", ui.Value("fest validate"))
	fmt.Fprintf(&steps, "  4. When the plan is solid: %s\n", ui.Value("fest promote"))
	steps.WriteString("\n")
	steps.WriteString(ui.Dim("Run 'fest next' again after each step for the updated instruction."))
	steps.WriteString("\n")

	data := struct {
		InstructionHeader string
		Header            string
		InfoSection       string
		MarkerSection     string
		NextStepsSection  string
	}{
		InstructionHeader: guidance.InstructionHeader,
		Header:            ui.H1("Festival Planning"),
		InfoSection:       info.String(),
		MarkerSection:     markers.String(),
		NextStepsSection:  steps.String(),
	}

	var buf bytes.Buffer
	_ = agent.MustGet("next/festival_planning").Execute(&buf, data)
	return buf.String()
}
