package task

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

var (
	operatorPromptIn  io.Reader = os.Stdin
	operatorPromptOut io.Writer = os.Stdout
)

// confirmDeferral asks the operator to type the task number rather than y.
// There is no default: a wrong number, y, or an empty line all cancel.
func confirmDeferral(taskID string, task *progress.TaskProgress) bool {
	number := taskNumberFrom(taskID)

	var prompt strings.Builder
	prompt.WriteString("\n")
	prompt.WriteString(ui.Warning("Deferring a blocker lets the festival move on without this task.") + "\n")
	prompt.WriteString("  The task stays blocked. fest next will bring it back at the end.\n\n")
	prompt.WriteString("  Task:    " + ui.Value(taskID, ui.TaskColor) + "\n")
	prompt.WriteString("  Blocker: " + task.BlockerMessage + "\n")
	prompt.WriteString("  Blocked: " + humanDuration(task.BlockedAt) + "\n")

	if len(task.BlockerAttempts) == 0 {
		prompt.WriteString("\n")
		prompt.WriteString(ui.Warning("  No unblock attempts were recorded. Consider 'fest task unblock --note' instead.") + "\n")
	} else {
		prompt.WriteString("  Tried:\n")
		for _, attempt := range task.BlockerAttempts {
			prompt.WriteString("    - " + attempt + "\n")
		}
	}

	prompt.WriteString("\nType the task number (" + number + ") to defer, anything else to cancel: ")

	_, _ = io.WriteString(operatorPromptOut, prompt.String())

	response, err := bufio.NewReader(operatorPromptIn).ReadString('\n')
	if err != nil && response == "" {
		return false
	}
	return strings.TrimSpace(response) == number
}

// taskNumberFrom returns the leading digit run of the task file name, which is
// the number the operator reads off the task path. deps.extractNumber parses
// the same prefix into an int for ordering, which cannot express the zero
// padding the operator has to type.
func taskNumberFrom(taskID string) string {
	base := filepath.Base(filepath.ToSlash(taskID))
	end := 0
	for end < len(base) && base[end] >= '0' && base[end] <= '9' {
		end++
	}
	return base[:end]
}

func humanDuration(since *time.Time) string {
	return humanDurationAt(time.Now(), since)
}

// humanDurationAt takes the reference time so a rendered age can be asserted
// against a golden without depending on the wall clock.
func humanDurationAt(now time.Time, since *time.Time) string {
	if since == nil {
		return "unknown"
	}
	return ui.FormatDuration(int(now.Sub(*since).Minutes()))
}

// gitUserName resolves the deferring operator's name. Any failure yields an
// empty string rather than a fallback that could name an agent service
// account (D001).
func gitUserName(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "git", "config", "--get", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
