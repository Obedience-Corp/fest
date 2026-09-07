package selection

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// KindFestivalPlanning is the step kind fest next reports for a festival that
// has no executable work yet because its own plan is still unwritten.
const KindFestivalPlanning = "festival_planning"

// FestivalPlanningResult carries everything an agent needs to plan a festival
// from the fest next output alone: what the festival is for, which documents
// still hold template markers and what each marker asks for, and the commands
// that turn the scaffold into an executable plan.
//
// It is the payload behind NextTaskResult.FestivalPlanning. Its presence, and
// the matching NextTaskResult.Kind, are what tell a consumer that fest next
// returned a planning step rather than a task. Task stays nil and
// FestivalComplete stays false.
type FestivalPlanningResult struct {
	// Status is the festival's lifecycle status, always "planning" today.
	Status string `json:"status"`

	// PhaseCount is the number of numbered phase directories in the festival.
	PhaseCount int `json:"phase_count"`

	// Goal is the festival's stated goal, empty while it is still a marker.
	Goal string `json:"goal,omitempty"`

	// MarkerTotal is the number of unfilled template markers across all files.
	MarkerTotal int `json:"marker_total"`

	// MarkerFiles lists each file that still holds markers, with the hint text
	// of every marker in it.
	MarkerFiles []PlanningMarkerFile `json:"marker_files,omitempty"`

	// NextCommands lists the commands that build the plan, in order. Filling
	// markers is an edit rather than a command and is not listed here.
	NextCommands []string `json:"next_commands,omitempty"`
}

// PlanningMarkerFile is one file with unfilled template markers.
type PlanningMarkerFile struct {
	// File is the path relative to the festival root.
	File string `json:"file"`
	// Count is the number of unfilled markers in that file.
	Count int `json:"count"`
	// Markers is the hint text of each unfilled marker, in file order.
	Markers []PlanningMarker `json:"markers,omitempty"`
}

// PlanningMarker is a single unfilled marker and what it asks the author for.
type PlanningMarker struct {
	// Line is the 1-indexed line the marker sits on.
	Line int `json:"line"`
	// Hint is the marker as written, e.g. "[REPLACE: the outcome]".
	Hint string `json:"hint"`
}

// formatTextFestivalPlanning renders the planning step. Verbose output uses the
// same renderer: the step already inlines everything it has, so there is
// nothing for a verbose mode to expand.
func formatTextFestivalPlanning(result *NextTaskResult) string {
	p := result.FestivalPlanning

	var sb strings.Builder
	sb.WriteString(guidance.InstructionHeader)
	sb.WriteString("\n")
	sb.WriteString(ui.H1("Festival Planning"))
	sb.WriteString("\n")
	writePlanningIntro(&sb, p)
	writePlanningIdentity(&sb, result, p)
	writePlanningMarkers(&sb, p)
	writePlanningBuildSteps(&sb, p)
	return sb.String()
}

// writePlanningIntro states what this step is before anything else, so an agent
// reading only this output knows it is planning rather than executing.
func writePlanningIntro(sb *strings.Builder, p *FestivalPlanningResult) {
	if p.MarkerTotal > 0 {
		sb.WriteString("This festival has no phases yet, and its documents are still templates.\n")
		sb.WriteString("Plan it before executing it: write the documents, then add the phases.\n\n")
		return
	}
	sb.WriteString("The festival documents are written, but it has no phases yet.\n")
	sb.WriteString("Plan it before executing it: add the phases that carry the work.\n\n")
}

func writePlanningIdentity(sb *strings.Builder, result *NextTaskResult, p *FestivalPlanningResult) {
	if result.Location != nil && result.Location.FestivalPath != "" {
		ui.WriteLabelValue(sb, "Festival", ui.Value(filepath.Base(result.Location.FestivalPath), ui.FestivalColor))
		ui.WriteLabelValue(sb, "Path", ui.Dim(result.Location.FestivalPath))
	}
	ui.WriteLabelValue(sb, "Status", ui.Value(p.Status))
	ui.WriteLabelValue(sb, "Phases", ui.Value(fmt.Sprintf("%d", p.PhaseCount)))
	if p.Goal != "" {
		ui.WriteLabelValue(sb, "Goal", ui.Info(p.Goal))
	}
}

// writePlanningMarkers lists every unfilled marker by hint text. This is the
// only variable-length part of the step: an agent needs each hint to know what
// to write, so the list is never truncated.
func writePlanningMarkers(sb *strings.Builder, p *FestivalPlanningResult) {
	if len(p.MarkerFiles) == 0 {
		return
	}

	fmt.Fprintf(sb, "\n%s\n", ui.H2(fmt.Sprintf("Unfilled Markers (%d in %d files)",
		p.MarkerTotal, len(p.MarkerFiles))))
	sb.WriteString("Replace each marker below with real content. Agents edit these files\n")
	sb.WriteString("directly. Humans can run 'fest wizard fill .' for the same job, and\n")
	sb.WriteString("'fest create festival --markers' fills them at creation time.\n")

	for _, file := range p.MarkerFiles {
		fmt.Fprintf(sb, "\n  %s\n", ui.Value(file.File, ui.TaskColor))
		for _, marker := range file.Markers {
			fmt.Fprintf(sb, "    %s  %s\n", ui.Dim(fmt.Sprintf("%4d", marker.Line)), marker.Hint)
		}
	}
}

// writePlanningBuildSteps gives the plan-building loop in fest's own vocabulary,
// with the methodology read just in time rather than quoted here.
func writePlanningBuildSteps(sb *strings.Builder, p *FestivalPlanningResult) {
	step := 1
	number := func() string {
		s := fmt.Sprintf("  %d.", step)
		step++
		return s
	}

	fmt.Fprintf(sb, "\n%s\n", ui.H2("Build The Plan"))
	if p.MarkerTotal > 0 {
		fmt.Fprintf(sb, "%s Fill the markers above with real content.\n", number())
	}
	fmt.Fprintf(sb, "%s Read the methodology just in time:\n", number())
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest understand planning"))
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest understand structure"))
	fmt.Fprintf(sb, "%s Decide which phases this festival needs.\n", number())
	fmt.Fprintf(sb, "%s Create each phase:\n", number())
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest create phase --name PHASE_NAME --type TYPE"))
	fmt.Fprintf(sb, "       %s\n", ui.Dim("types: ingest, planning, research, implementation, review"))
	fmt.Fprintf(sb, "%s Fill each phase with sequences and their task files:\n", number())
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest create sequence --name SEQUENCE_NAME"))
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest create task --name TASK_NAME"))
	fmt.Fprintf(sb, "%s Check the structure:\n", number())
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest validate"))
	fmt.Fprintf(sb, "%s When the plan is solid, promote it:\n", number())
	fmt.Fprintf(sb, "       %s\n", ui.Value("fest promote"))
	sb.WriteString("\n")
	sb.WriteString(ui.Dim("Run 'fest next' again after each step for the updated instruction."))
	sb.WriteString("\n")
}
