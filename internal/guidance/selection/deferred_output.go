package selection

import (
	"fmt"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// buildDeferralSection is the one block every task rendering inserts under the
// task, before the guidance that follows it. It carries the sweep explanation
// for a handed-back blocker, the selected task's deferred dependencies, and any
// operator note. It is empty when the result has none of those, which is what
// keeps a festival with nothing deferred rendering identically.
func buildDeferralSection(result *NextTaskResult) string {
	if result == nil {
		return ""
	}

	var sections []string
	if sweep := buildSweepSection(result.Sweep); sweep != "" {
		sections = append(sections, sweep)
	}
	if blockers := buildDeferredBlockerLines(result.DeferredBlockers); blockers != "" {
		sections = append(sections, blockers)
	}
	if notes := buildOperatorNotesSection(result.OperatorNotes); notes != "" {
		sections = append(sections, notes)
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n")
}

// buildDeferredBlockerLines prints one line per deferred dependency: the task,
// the executor's blocker message in quotes, and the date the operator deferred
// it. The separator is a colon, not an emdash, which the repository does not
// put in output.
func buildDeferredBlockerLines(refs []TaskRef) string {
	if len(refs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, ref := range refs {
		line := "Deferred blocker: " + ref.Task
		if ref.BlockerMessage != "" {
			line += ": \"" + ref.BlockerMessage + "\""
		}
		if day := deferredDay(ref.DeferredAt); day != "" {
			line += " (deferred " + day + ")"
		}
		sb.WriteString(ui.Warning(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

// deferredDay reduces an RFC 3339 stamp to the calendar day the rendered line
// shows. An unparseable stamp yields no date rather than a wrong one.
func deferredDay(stamp string) string {
	if stamp == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.DateOnly)
}

func buildOperatorNotesSection(notes []string) string {
	if len(notes) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, note := range notes {
		sb.WriteString(ui.Info("Operator note: " + note))
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildSweepSection explains why a task the executor already blocked is being
// offered again. The completed-since list comes last because those completions
// are what might have changed the situation.
func buildSweepSection(info *SweepInfo) string {
	if info == nil || len(info.Remaining) > 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", ui.H3(fmt.Sprintf("Sweep %d: revisiting a deferred blocker", info.Number)))

	if info.BlockerMessage != "" {
		fmt.Fprintf(&sb, "  %s %s\n", ui.Label("Blocker:"), ui.Dim(info.BlockerMessage))
	}
	for i, attempt := range info.Attempts {
		label := ui.Label("Tried:  ")
		if i > 0 {
			label = strings.Repeat(" ", len("Tried:  "))
		}
		fmt.Fprintf(&sb, "  %s %s\n", label, ui.Dim(attempt))
	}
	if info.DeferralReason != "" {
		fmt.Fprintf(&sb, "  %s %s\n", ui.Label("Deferred because:"), ui.Dim(info.DeferralReason))
	}

	if len(info.CompletedSince) > 0 {
		fmt.Fprintf(&sb, "\n  %s\n", ui.Label("Completed since it was deferred:"))
		for _, task := range info.CompletedSince {
			fmt.Fprintf(&sb, "    - %s\n", ui.Dim(task))
		}
	}

	return sb.String()
}

// buildSweepRemainSection lists the blockers behind the terminal sweep
// sentence, which the result's Reason already carries word for word.
func buildSweepRemainSection(info *SweepInfo) string {
	if info == nil || len(info.Remaining) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, blocker := range info.Remaining {
		fmt.Fprintf(&sb, "  %s %s %s\n", ui.StateIcon("blocked"),
			ui.Value(blocker.Task, ui.TaskColor), ui.Dim(blocker.BlockerMessage))
		if blocker.DeferralReason != "" {
			fmt.Fprintf(&sb, "      %s\n", ui.Dim("deferred: "+blocker.DeferralReason))
		}
	}
	return sb.String()
}

// buildBlockedSection is what an executor sees after it reports a blocker and
// runs fest next hoping to move on: the blockers holding the festival, and the
// one sentence about what blocked means. The sentence lives in
// internal/guidance so this surface and the orchestration agent context can
// never say different things.
func buildBlockedSection(blocked []TaskRef) string {
	if len(blocked) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(ui.H3("Blocked"))
	sb.WriteString("\n")
	for _, ref := range blocked {
		fmt.Fprintf(&sb, "  %s %s %s\n", ui.StateIcon("blocked"),
			ui.Value(ref.Task, ui.TaskColor), ui.Dim(ref.BlockerMessage))
	}

	fmt.Fprintf(&sb, "\n%s\n", ui.Warning(guidance.ExecutorBlockerPolicy))
	return sb.String()
}
