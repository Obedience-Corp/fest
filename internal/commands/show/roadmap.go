package show

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/orchestration"
)

type roadmapOutput struct {
	Festival *FestivalInfo                `json:"festival"`
	Roadmap  *orchestration.ExecutionPlan `json:"roadmap"`
}

// emitRoadmapJSON outputs the festival info and full execution plan as wrapped JSON.
func emitRoadmapJSON(ctx context.Context, festival *FestivalInfo, campaignRoot string) error {
	plan, err := buildRoadmap(ctx, festival.Path)
	if err != nil {
		return err
	}

	output := &roadmapOutput{
		Festival: festival,
		Roadmap:  plan,
	}
	if campaignRoot != "" {
		output.Festival = toDisplayFestival(festival, campaignRoot)
	}

	if err := shared.EncodeJSON(os.Stdout, output); err != nil {
		return errors.Wrap(err, "encoding roadmap JSON")
	}
	return nil
}

// emitRoadmapText renders the execution plan as a human-readable tree.
func emitRoadmapText(ctx context.Context, festival *FestivalInfo) error {
	plan, err := buildRoadmap(ctx, festival.Path)
	if err != nil {
		return err
	}

	fmt.Print(renderRoadmapText(festival.Name, plan))
	return nil
}

// buildRoadmap creates an ExecutionPlan for the given festival path.
func buildRoadmap(ctx context.Context, festivalPath string) (*orchestration.ExecutionPlan, error) {
	builder := orchestration.NewPlanBuilder(festivalPath)
	plan, err := builder.BuildPlan(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "building execution plan")
	}
	return plan, nil
}

// renderRoadmapText formats an ExecutionPlan as a readable tree string.
func renderRoadmapText(festivalName string, plan *orchestration.ExecutionPlan) string {
	var b strings.Builder

	title := fmt.Sprintf("Festival Roadmap: %s", festivalName)
	b.WriteString("\n" + title + "\n")
	b.WriteString(strings.Repeat("=", len(title)) + "\n")

	if len(plan.Phases) == 0 {
		b.WriteString("\n  (no phases found)\n")
		return b.String()
	}

	for i, phase := range plan.Phases {
		fmt.Fprintf(&b, "\nPhase %s (%s)\n", phase.Name, phase.Status)

		lastPhase := i == len(plan.Phases)-1
		for j, seq := range phase.Sequences {
			lastSeq := j == len(phase.Sequences)-1
			seqPrefix := "├─"
			seqCont := "│ "
			if lastSeq {
				seqPrefix = "└─"
				seqCont = "  "
			}

			fmt.Fprintf(&b, "  %s %s (%d tasks)\n", seqPrefix, seq.Name, seq.TotalTasks)

			for k, step := range seq.Steps {
				lastStep := k == len(seq.Steps)-1
				stepPrefix := seqCont + "   "
				_ = lastStep
				_ = lastPhase

				if step.Parallel && len(step.Tasks) > 1 {
					fmt.Fprintf(&b, "  %s Step %d: [parallel]\n", stepPrefix, step.Number)
					for t, task := range step.Tasks {
						taskPrefix := "├─"
						if t == len(step.Tasks)-1 {
							taskPrefix = "└─"
						}
						marker := statusMarker(task.IsGate)
						fmt.Fprintf(&b, "  %s   %s %s%s %s\n", stepPrefix, taskPrefix, marker, task.Name, dotPad(task.Name, task.Status, marker))
					}
				} else if len(step.Tasks) == 1 {
					task := step.Tasks[0]
					marker := statusMarker(task.IsGate)
					fmt.Fprintf(&b, "  %s Step %d: %s%s %s\n", stepPrefix, step.Number, marker, task.Name, dotPad(task.Name, task.Status, marker))
				}
			}
		}
	}

	// Summary line
	if plan.Summary != nil {
		s := plan.Summary
		fmt.Fprintf(&b, "\nSummary: %d phases, %d sequences, %d tasks",
			s.TotalPhases, s.TotalSequences, s.TotalTasks)
		if s.ParallelGroups > 0 {
			fmt.Fprintf(&b, ", %d parallel groups", s.ParallelGroups)
		}
		if s.QualityGates > 0 {
			fmt.Fprintf(&b, ", %d quality gates", s.QualityGates)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// statusMarker returns a prefix marker for gate tasks.
func statusMarker(isGate bool) string {
	if isGate {
		return "[gate] "
	}
	return ""
}

// dotPad creates a dot-padded status alignment string.
func dotPad(name, status, marker string) string {
	nameLen := len(marker) + len(name)
	padLen := 40 - nameLen
	if padLen < 2 {
		padLen = 2
	}
	return strings.Repeat(".", padLen) + " " + status
}
