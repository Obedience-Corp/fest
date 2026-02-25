package selection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/embedded/templates/agent"
	"github.com/Obedience-Corp/fest/internal/context"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// FormatText formats the result as human-readable text
// If showInlineContext is true, task content summary is included in the output
func FormatText(result *NextTaskResult, showInlineContext bool) string {
	switch {
	case result.FestivalComplete:
		return formatTextComplete(result)
	case result.Planning != nil:
		return formatTextPlanning(result)
	case result.Task == nil:
		return formatTextNoTask(result)
	default:
		return formatTextTask(result, showInlineContext)
	}
}

// FormatJSON formats the result as JSON
func FormatJSON(result *NextTaskResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatVerbose formats the result with additional details
// If showInlineContext is true, task content summary is included in the output
func FormatVerbose(result *NextTaskResult, showInlineContext bool) string {
	switch {
	case result.FestivalComplete:
		return formatVerboseComplete(result)
	case result.Planning != nil:
		return formatVerbosePlanning(result)
	case result.Task == nil:
		return formatVerboseNoTask(result)
	default:
		return formatVerboseTask(result, showInlineContext)
	}
}

func formatTextComplete(result *NextTaskResult) string {
	var reasonLine string
	if result.Reason != "" {
		reasonLine = labelValue("Reason", ui.Info(result.Reason))
	}

	data := struct {
		Header     string
		Message    string
		ReasonLine string
	}{
		Header:     ui.H2("Festival Complete"),
		Message:    ui.Success("All tasks have been completed."),
		ReasonLine: reasonLine,
	}

	var buf bytes.Buffer
	agent.MustGet("next/complete").Execute(&buf, data)
	return buf.String()
}

func formatTextNoTask(result *NextTaskResult) string {
	var reasonLine string
	if result.Reason != "" {
		reasonLine = labelValue("Reason", ui.Info(result.Reason))
	}

	var locationSection string
	if result.Location != nil {
		var sb strings.Builder
		sb.WriteString(ui.H3("Location"))
		sb.WriteString("\n")
		ui.WriteLabelValue(&sb, "Festival", ui.Dim(result.Location.FestivalPath))
		if result.Location.PhasePath != "" {
			ui.WriteLabelValue(&sb, "Phase", ui.Dim(filepath.Base(result.Location.PhasePath)))
		}
		if result.Location.SequencePath != "" {
			ui.WriteLabelValue(&sb, "Sequence", ui.Dim(filepath.Base(result.Location.SequencePath)))
		}
		locationSection = sb.String()
	}

	data := struct {
		Header          string
		ReasonLine      string
		LocationSection string
	}{
		Header:          ui.H2("No Tasks Available"),
		ReasonLine:      reasonLine,
		LocationSection: locationSection,
	}

	var buf bytes.Buffer
	agent.MustGet("next/no_task").Execute(&buf, data)
	return buf.String()
}

func formatTextPlanning(result *NextTaskResult) string {
	p := result.Planning
	var sb strings.Builder

	// Header
	sb.WriteString(ui.H1("Planning Phase"))
	sb.WriteString("\n")

	// Phase info
	ui.WriteLabelValue(&sb, "Phase", ui.Value(p.PhaseName, ui.PhaseColor))
	ui.WriteLabelValue(&sb, "Type", ui.Value(p.PhaseType))

	// Progress
	if p.Progress != nil {
		progressStr := fmt.Sprintf("%.0f%% (%d/%d objectives)",
			p.Progress.Percentage,
			p.Progress.ResolvedObjectives,
			p.Progress.TotalObjectives)
		if p.GraduationReady {
			ui.WriteLabelValue(&sb, "Progress", ui.Success(progressStr+" - Ready to promote!"))
		} else {
			ui.WriteLabelValue(&sb, "Progress", ui.Info(progressStr))
		}
	}

	sb.WriteString("\n")

	// Objectives by category
	if len(p.Objectives) > 0 {
		sb.WriteString(ui.H2("Objectives from PHASE_GOAL.md"))
		sb.WriteString("\n")

		// Group by category
		categories := map[string][]*PlanningObjective{
			"question":  {},
			"decision":  {},
			"artifact":  {},
			"objective": {},
		}
		for _, obj := range p.Objectives {
			categories[obj.Category] = append(categories[obj.Category], obj)
		}

		// Display each category with objectives
		categoryTitles := map[string]string{
			"question":  "Questions to Answer",
			"decision":  "Decisions to Make",
			"artifact":  "Artifacts to Produce",
			"objective": "Objectives",
		}
		categoryOrder := []string{"question", "decision", "artifact", "objective"}

		for _, cat := range categoryOrder {
			objs := categories[cat]
			if len(objs) == 0 {
				continue
			}
			sb.WriteString(ui.H3(categoryTitles[cat]))
			sb.WriteString("\n")
			for _, obj := range objs {
				icon := ui.StateIcon("pending")
				if obj.Resolved {
					icon = ui.StateIcon("completed")
				}
				sb.WriteString(fmt.Sprintf("  %s %s\n", icon, obj.Text))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(ui.Dim("No planning objectives found in PHASE_GOAL.md"))
		sb.WriteString("\n")
		sb.WriteString(ui.Info("Add objectives as checkboxes under 'Planning Objectives' section"))
		sb.WriteString("\n\n")
	}

	// Graduation hint or next steps
	if p.GraduationReady {
		sb.WriteString(ui.H2("Next Step"))
		sb.WriteString("\n")
		sb.WriteString(ui.Success("All objectives resolved! Ready to promote."))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("  Run: %s\n\n", ui.Value("fest promote")))
		sb.WriteString(ui.Info("This will promote the festival to the next lifecycle status."))
		sb.WriteString("\n")
	} else {
		sb.WriteString(ui.H2("Suggested Actions"))
		sb.WriteString("\n")
		sb.WriteString("  - Explore the problem space and document findings\n")
		sb.WriteString("  - Update PHASE_GOAL.md checkboxes as objectives are resolved\n")
		sb.WriteString("  - Create topic directories for deep exploration\n")
		sb.WriteString("  - Run 'fest promote' when all objectives are complete\n")
	}

	return sb.String()
}

func formatTextTask(result *NextTaskResult, showInlineContext bool) string {
	// Check if this is a gate task and show inline gate content
	if gateSection := buildGateSection(result.Task); gateSection != "" {
		return gateSection
	}

	// Build parallel tasks section if present
	var parallelSection string
	if len(result.ParallelTasks) > 0 {
		var sb strings.Builder
		sb.WriteString(ui.H3("Parallel Tasks"))
		sb.WriteString("\n")
		for _, task := range result.ParallelTasks {
			sb.WriteString(fmt.Sprintf("  - %s %s\n", ui.Value(task.Name, ui.TaskColor), ui.Dim(task.SequenceName)))
		}
		parallelSection = sb.String()
	}

	// Build autonomy line if present
	var autonomyLine string
	if result.Task.AutonomyLevel != "" {
		var sb strings.Builder
		ui.WriteLabelValue(&sb, "Autonomy", ui.Value(result.Task.AutonomyLevel))
		autonomyLine = strings.TrimSuffix(sb.String(), "\n")
	}

	// Build progress line if available
	var progressLine string
	if result.Progress != nil {
		progressLine = labelValue("Progress", ui.Info(fmt.Sprintf("%.1f%% (%d/%d tasks)",
			result.Progress.Percentage,
			result.Progress.CompletedTasks,
			result.Progress.TotalTasks)))
	}

	// When inline context is enabled, use layered prompts
	var layeredGoalsSection string
	var taskContentSection string
	var festivalRulesSection string
	var contextSection string

	// Build relative path for display
	taskRelPath := filepath.Join(result.Task.PhaseName, result.Task.SequenceName, result.Task.Name+".md")

	if showInlineContext {
		// Extract and format layered goals
		goals := extractLayeredGoals(result.Location, result.Task)
		layeredGoalsSection = buildLayeredGoalsSection(goals)

		// Get full task content (no truncation)
		taskContentSection = buildFullTaskContentSection(result.Task.Path)

		// Hint to run fest rules instead of dumping inline
		if result.Location != nil && result.Location.FestivalPath != "" {
			rulesPath := filepath.Join(result.Location.FestivalPath, "FESTIVAL_RULES.md")
			if _, err := os.Stat(rulesPath); err == nil {
				festivalRulesSection = "\n## Festival Rules\n\nReview festival rules before starting: `fest rules`\n"
			}
		}

		// Don't show context files section in layered mode
		contextSection = ""
	} else {
		// Standard mode: show context file paths only
		contextSection = buildContextSection(result.Location, result.Task, false)
	}

	data := struct {
		Header               string
		TaskLine             string
		PathLine             string
		SequenceLine         string
		PhaseLine            string
		AutonomyLine         string
		ProgressLine         string
		ParallelSection      string
		ProgressCmd          string
		ContextSection       string
		LayeredGoalsSection  string
		TaskContentSection   string
		FestivalRulesSection string
		ShowInlineContext    bool
	}{
		Header:               ui.H1("Next Task"),
		TaskLine:             labelValue("Task", ui.Value(result.Task.Name, ui.TaskColor)),
		PathLine:             labelValue("Path", ui.Dim(taskRelPath)),
		SequenceLine:         labelValue("Sequence", ui.Value(result.Task.SequenceName, ui.SequenceColor)),
		PhaseLine:            labelValue("Phase", ui.Value(result.Task.PhaseName, ui.PhaseColor)),
		AutonomyLine:         autonomyLine,
		ProgressLine:         progressLine,
		ParallelSection:      parallelSection,
		ProgressCmd:          ui.Value(guidance.FormatProgressCommand(taskRelPath)),
		ContextSection:       contextSection,
		LayeredGoalsSection:  layeredGoalsSection,
		TaskContentSection:   taskContentSection,
		FestivalRulesSection: festivalRulesSection,
		ShowInlineContext:    showInlineContext,
	}

	var buf bytes.Buffer
	agent.MustGet("next/task").Execute(&buf, data)
	return buf.String()
}

// buildContextSection creates the context files section showing goal files
// If showSummaries is true, includes first paragraph excerpts from each goal file
func buildContextSection(loc *LocationInfo, task *TaskInfo, showSummaries bool) string {
	var sb strings.Builder
	sb.WriteString(ui.H3("Context Files"))
	sb.WriteString("\n")

	// Festival goal
	if loc != nil && loc.FestivalPath != "" {
		festivalGoal := filepath.Join(loc.FestivalPath, "FESTIVAL_GOAL.md")
		sb.WriteString(fmt.Sprintf("  - %s\n", ui.Dim(festivalGoal)))
		if showSummaries {
			if summary := extractFirstParagraph(festivalGoal); summary != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", ui.Dim(summary)))
			}
		}
	}

	// Phase goal
	if task.PhasePath != "" {
		phaseGoal := filepath.Join(task.PhasePath, "PHASE_GOAL.md")
		sb.WriteString(fmt.Sprintf("  - %s\n", ui.Dim(phaseGoal)))
		if showSummaries {
			if summary := extractFirstParagraph(phaseGoal); summary != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", ui.Dim(summary)))
			}
		}
	}

	// Sequence goal
	if task.SequencePath != "" {
		sequenceGoal := filepath.Join(task.SequencePath, "SEQUENCE_GOAL.md")
		sb.WriteString(fmt.Sprintf("  - %s\n", ui.Dim(sequenceGoal)))
		if showSummaries {
			if summary := extractFirstParagraph(sequenceGoal); summary != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", ui.Dim(summary)))
			}
		}
	}

	return sb.String()
}

// extractFirstParagraph reads a markdown file and extracts its first non-header paragraph.
// Returns a truncated version if the paragraph is too long.
func extractFirstParagraph(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	var paragraphLines []string
	inFrontmatter := false
	frontmatterDone := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip frontmatter
		if trimmed == "---" {
			if !frontmatterDone {
				inFrontmatter = !inFrontmatter
				if !inFrontmatter {
					frontmatterDone = true
				}
			}
			continue
		}
		if inFrontmatter {
			continue
		}

		// Skip headers
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Skip empty lines at the start
		if trimmed == "" && len(paragraphLines) == 0 {
			continue
		}

		// End of paragraph
		if trimmed == "" && len(paragraphLines) > 0 {
			break
		}

		paragraphLines = append(paragraphLines, trimmed)
	}

	if len(paragraphLines) == 0 {
		return ""
	}

	paragraph := strings.Join(paragraphLines, " ")

	// Truncate if too long
	const maxLen = 120
	if len(paragraph) > maxLen {
		paragraph = paragraph[:maxLen-3] + "..."
	}

	return paragraph
}

// buildTaskContentSection reads the task file and formats its content for inline display.
// This helps prevent agents from batch-completing tasks without reading them.
func buildTaskContentSection(taskPath string) string {
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return ""
	}

	// Strip frontmatter
	body := stripFrontmatter(string(content))

	// Truncate if too long
	const maxContentLen = 2000
	if len(body) > maxContentLen {
		body = body[:maxContentLen-20] + "\n\n... (truncated)"
	}

	if strings.TrimSpace(body) == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(ui.H3("Task Content"))
	sb.WriteString("\n")
	sb.WriteString(ui.Dim(strings.Repeat("─", 60)))
	sb.WriteString("\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString(ui.Dim(strings.Repeat("─", 60)))
	sb.WriteString("\n")

	return sb.String()
}

// stripFrontmatter removes YAML frontmatter from markdown content
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}

	// Find the closing ---
	rest := content[3:]
	idx := strings.Index(rest, "---")
	if idx == -1 {
		return content
	}

	// Return content after frontmatter
	return strings.TrimSpace(rest[idx+3:])
}

// LayeredGoals holds extracted primary goals from the festival hierarchy
type LayeredGoals struct {
	FestivalGoal string
	PhaseGoal    string
	SequenceGoal string
}

// extractLayeredGoals extracts primary goals from all goal files.
func extractLayeredGoals(loc *LocationInfo, task *TaskInfo) *LayeredGoals {
	goals := &LayeredGoals{}

	if loc != nil && loc.FestivalPath != "" {
		path := filepath.Join(loc.FestivalPath, "FESTIVAL_GOAL.md")
		if content, err := os.ReadFile(path); err == nil {
			goals.FestivalGoal = context.ExtractPrimaryGoal(content)
		}
	}
	if task.PhasePath != "" {
		path := filepath.Join(task.PhasePath, "PHASE_GOAL.md")
		if content, err := os.ReadFile(path); err == nil {
			goals.PhaseGoal = context.ExtractPrimaryGoal(content)
		}
	}
	if task.SequencePath != "" {
		path := filepath.Join(task.SequencePath, "SEQUENCE_GOAL.md")
		if content, err := os.ReadFile(path); err == nil {
			goals.SequenceGoal = context.ExtractPrimaryGoal(content)
		}
	}
	return goals
}

// buildLayeredGoalsSection creates the hierarchical goal context section.
func buildLayeredGoalsSection(goals *LayeredGoals) string {
	if goals == nil {
		return ""
	}
	hasGoals := goals.FestivalGoal != "" || goals.PhaseGoal != "" || goals.SequenceGoal != ""
	if !hasGoals {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Context about the task you will be doing:\n")
	if goals.FestivalGoal != "" {
		sb.WriteString(fmt.Sprintf("Festival Goal: %s\n", goals.FestivalGoal))
	}
	if goals.PhaseGoal != "" {
		sb.WriteString(fmt.Sprintf("Phase Goal: %s\n", goals.PhaseGoal))
	}
	if goals.SequenceGoal != "" {
		sb.WriteString(fmt.Sprintf("Sequence Goal: %s\n", goals.SequenceGoal))
	}
	return sb.String()
}

// buildFullTaskContentSection reads the entire task file without truncation.
func buildFullTaskContentSection(taskPath string) string {
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return ""
	}

	body := stripFrontmatter(string(content))
	if strings.TrimSpace(body) == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Task Document\n\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	return sb.String()
}

// buildFestivalRulesSection reads and includes FESTIVAL_RULES.md content.

// labelValue formats a label-value pair without trailing newline
func labelValue(label, value string) string {
	var sb strings.Builder
	ui.WriteLabelValue(&sb, label, value)
	return strings.TrimSuffix(sb.String(), "\n")
}

func formatVerboseComplete(result *NextTaskResult) string {
	var sb strings.Builder

	sb.WriteString(ui.H2("Festival Complete"))
	sb.WriteString("\n")
	sb.WriteString(ui.Success("All tasks in the festival have been completed."))
	sb.WriteString("\n")
	sb.WriteString(ui.Info("Congratulations on finishing the festival!"))
	if result.Reason != "" {
		sb.WriteString("\n")
		ui.WriteLabelValue(&sb, "Reason", ui.Info(result.Reason))
	}

	return sb.String()
}

func formatVerboseNoTask(result *NextTaskResult) string {
	var sb strings.Builder

	sb.WriteString(ui.H2("No Tasks Available"))
	sb.WriteString("\n")
	if result.Reason != "" {
		ui.WriteLabelValue(&sb, "Reason", ui.Info(result.Reason))
	}

	if result.Location != nil {
		sb.WriteString("\n")
		sb.WriteString(ui.H3("Location"))
		sb.WriteString("\n")
		ui.WriteLabelValue(&sb, "Festival", ui.Dim(result.Location.FestivalPath))
		if result.Location.PhasePath != "" {
			ui.WriteLabelValue(&sb, "Phase", ui.Dim(filepath.Base(result.Location.PhasePath)))
		}
		if result.Location.SequencePath != "" {
			ui.WriteLabelValue(&sb, "Sequence", ui.Dim(filepath.Base(result.Location.SequencePath)))
		}
	}

	return sb.String()
}

func formatVerbosePlanning(result *NextTaskResult) string {
	// For verbose mode, use the same output as text mode
	// Planning phases don't need additional verbose details
	return formatTextPlanning(result)
}

func formatVerboseTask(result *NextTaskResult, showInlineContext bool) string {
	var sb strings.Builder

	sb.WriteString(ui.H1("Next Task"))
	sb.WriteString("\n")
	writeTaskDetails(&sb, result.Task)
	writeTaskLocation(&sb, result.Task)
	writeTaskProperties(&sb, result.Task)
	writeTaskDependencies(&sb, result.Task.Dependencies)
	writeRecommendation(&sb, result.Reason)
	writeParallelTasks(&sb, result.ParallelTasks)
	writeCurrentLocation(&sb, result.Location)

	// Add task content section if inline context is enabled
	if showInlineContext {
		taskContent := buildTaskContentSection(result.Task.Path)
		if taskContent != "" {
			sb.WriteString("\n")
			sb.WriteString(taskContent)
		}
	}

	return sb.String()
}

func writeTaskDetails(sb *strings.Builder, task *TaskInfo) {
	sb.WriteString(ui.H2("Task Details"))
	sb.WriteString("\n")
	ui.WriteLabelValue(sb, "Task", ui.Value(task.Name, ui.TaskColor))
	ui.WriteLabelValue(sb, "Number", ui.Value(fmt.Sprintf("%d", task.Number)))
	ui.WriteLabelValue(sb, "Path", ui.Dim(task.Path))
	sb.WriteString("\n")
}

func writeTaskLocation(sb *strings.Builder, task *TaskInfo) {
	sb.WriteString(ui.H2("Location"))
	sb.WriteString("\n")
	ui.WriteLabelValue(sb, "Phase", ui.Value(task.PhaseName, ui.PhaseColor))
	ui.WriteLabelValue(sb, "Sequence", ui.Value(task.SequenceName, ui.SequenceColor))
	sb.WriteString("\n")
}

func writeTaskProperties(sb *strings.Builder, task *TaskInfo) {
	if task.AutonomyLevel == "" && task.ParallelGroup == 0 {
		return
	}

	sb.WriteString(ui.H2("Properties"))
	sb.WriteString("\n")
	if task.AutonomyLevel != "" {
		ui.WriteLabelValue(sb, "Autonomy", ui.Value(task.AutonomyLevel))
	}
	if task.ParallelGroup > 0 {
		ui.WriteLabelValue(sb, "Parallel Group", ui.Value(fmt.Sprintf("%d", task.ParallelGroup)))
	}
	sb.WriteString("\n")
}

func writeTaskDependencies(sb *strings.Builder, deps []string) {
	if len(deps) == 0 {
		return
	}

	sb.WriteString(ui.H2("Dependencies"))
	sb.WriteString("\n")
	for _, dep := range deps {
		sb.WriteString(fmt.Sprintf("  %s %s\n", ui.StateIcon("completed"), ui.Info(dep)))
	}
	sb.WriteString("\n")
}

func writeRecommendation(sb *strings.Builder, reason string) {
	sb.WriteString(ui.H2("Recommendation"))
	sb.WriteString("\n")
	if reason == "" {
		sb.WriteString(ui.Dim("No recommendation available."))
	} else {
		sb.WriteString(ui.Info(reason))
	}
	sb.WriteString("\n\n")
}

func writeParallelTasks(sb *strings.Builder, tasks []*TaskInfo) {
	if len(tasks) == 0 {
		return
	}

	sb.WriteString(ui.H2("Parallel Tasks"))
	sb.WriteString("\n")
	for _, task := range tasks {
		sb.WriteString(fmt.Sprintf("  - %s\n", ui.Value(task.Name, ui.TaskColor)))
		sb.WriteString(fmt.Sprintf("    %s %s\n", ui.Label("Path"), ui.Dim(task.Path)))
		if task.AutonomyLevel != "" {
			sb.WriteString(fmt.Sprintf("    %s %s\n", ui.Label("Autonomy"), ui.Value(task.AutonomyLevel)))
		}
		sb.WriteString("\n")
	}
}

func writeCurrentLocation(sb *strings.Builder, loc *LocationInfo) {
	sb.WriteString(ui.H2("Current Location"))
	sb.WriteString("\n")
	if loc == nil {
		sb.WriteString(ui.Dim("Unknown location\n"))
		return
	}
	ui.WriteLabelValue(sb, "Festival", ui.Dim(filepath.Base(loc.FestivalPath)))
	if loc.PhasePath != "" {
		ui.WriteLabelValue(sb, "Phase", ui.Dim(filepath.Base(loc.PhasePath)))
	}
	if loc.SequencePath != "" {
		ui.WriteLabelValue(sb, "Sequence", ui.Dim(filepath.Base(loc.SequencePath)))
	}
}

// FormatShort formats a minimal one-line output
func FormatShort(result *NextTaskResult) string {
	if result.FestivalComplete {
		return "Festival complete"
	}
	if result.Task == nil {
		return "No tasks available"
	}
	return result.Task.Path
}

// FormatCD formats output suitable for shell cd command
func FormatCD(result *NextTaskResult) string {
	if result.Task == nil {
		return ""
	}
	// Return the directory containing the task file
	return filepath.Dir(result.Task.Path)
}

// buildGateSection checks if a task is a gate task and returns inline gate content.
// Returns empty string if the task is not a gate.
func buildGateSection(task *TaskInfo) string {
	if task == nil || task.Path == "" {
		return ""
	}

	// Read and parse frontmatter to detect gate type
	data, err := os.ReadFile(task.Path)
	if err != nil {
		return ""
	}

	fm, body, err := frontmatter.Parse(data)
	if err != nil || fm == nil {
		return ""
	}

	// Not a gate task
	if fm.GateType == "" {
		return ""
	}

	var sb strings.Builder

	// Header
	gateTitle := strings.ToUpper(string(fm.GateType[:1])) + string(fm.GateType[1:])
	sb.WriteString(ui.H1(fmt.Sprintf("Quality Gate: %s", gateTitle)))
	sb.WriteString("\n")

	// Task location
	taskRelPath := filepath.Join(task.PhaseName, task.SequenceName, task.Name+".md")
	sb.WriteString(labelValue("Task", ui.Value(task.Name, ui.TaskColor)))
	sb.WriteString("\n")
	sb.WriteString(labelValue("Path", ui.Dim(taskRelPath)))
	sb.WriteString("\n")
	sb.WriteString(labelValue("Type", ui.Value(fmt.Sprintf("gate (%s)", fm.GateType))))
	sb.WriteString("\n")

	// Gate content (strip frontmatter, show body)
	content := strings.TrimSpace(string(body))
	if content != "" {
		sb.WriteString(content)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(ui.Dim("Gate file could not be read. Run `fest gates` to evaluate gate criteria."))
		sb.WriteString("\n\n")
	}

	// Action hint
	sb.WriteString(ui.Dim("When complete, run: "))
	sb.WriteString(ui.Value("fest task completed"))
	sb.WriteString("\n")
	sb.WriteString(ui.Dim("When ready for the next task, run: "))
	sb.WriteString(ui.Value("fest next"))
	sb.WriteString("\n\n")

	return sb.String()
}
