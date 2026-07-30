package task

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"golang.org/x/term"
)

// resolveConfirmation decides how a consequential task mutation should proceed
// given the --yes and --json flags and whether stdin is an interactive terminal.
//
// It returns needPrompt=true only when the caller must run its own interactive
// Y/n prompt (no --yes, not JSON, stdin is a TTY). When yes is set it returns
// needPrompt=false with no error so the caller proceeds directly.
//
// When confirmation cannot be obtained it returns an error the caller must
// propagate: in JSON mode it first emits a structured "confirmation required"
// object to stdout; in a non-interactive terminal it returns a validation error
// directing the caller to --yes. This keeps agent invocations from silently
// mutating state without an explicit --yes.
func resolveConfirmation(yes, jsonOut bool, taskID, action, command string) (needPrompt bool, err error) {
	if yes {
		return false, nil
	}

	if jsonOut {
		result := map[string]any{
			"error":   "confirmation required",
			"task":    taskID,
			"message": "pass --yes to " + action + " non-interactively (e.g. '" + command + "')",
		}
		if encErr := shared.EncodeJSON(os.Stdout, result); encErr != nil {
			return false, errors.Wrap(encErr, "encoding JSON output")
		}
		return false, errors.Validation("confirmation required for "+action).
			WithField("task", taskID).
			WithHint("pass --yes for non-interactive use")
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.Validation("confirmation required for "+action).
			WithField("task", taskID).
			WithHint("pass --yes for non-interactive use")
	}

	return true, nil
}

// confirmCompletion asks the user to confirm task completion.
// Returns true if the user confirms.
func confirmCompletion(taskID string) bool {
	fmt.Println()
	fmt.Println(ui.Warning("Before marking this task complete, verify:"))
	fmt.Println("  - All acceptance criteria in the task document are met")
	fmt.Println("  - Quality gates pass (build, test, lint)")
	fmt.Println("  - You have reviewed your own changes")
	fmt.Println()
	fmt.Printf("  Task: %s\n", ui.Value(taskID, ui.TaskColor))
	fmt.Println()
	return promptYN("Are you sure this task is completed?")
}

// confirmBlocked asks the user to confirm reporting a blocker.
// Returns true if the user confirms.
func confirmBlocked(taskID, reason string) bool {
	fmt.Println()
	fmt.Println(ui.Warning("Reporting a blocker pauses work on this task."))
	fmt.Println("  The user will be notified and can help resolve it.")
	fmt.Println()
	fmt.Printf("  Task:   %s\n", ui.Value(taskID, ui.TaskColor))
	fmt.Printf("  Reason: %s\n", reason)
	fmt.Println()
	return promptYN("Are you stuck and do you need help from the user?")
}

// confirmReset asks the user to confirm resetting a task.
// Returns true if the user confirms.
func confirmReset(taskID string) bool {
	fmt.Println()
	fmt.Println(ui.Warning("Resetting a task clears ALL progress:"))
	fmt.Println("  - Status returns to pending")
	fmt.Println("  - Progress percentage resets to 0%")
	fmt.Println("  - Time tracking data is cleared")
	fmt.Println("  - Any blocker information is removed")
	fmt.Println()
	fmt.Printf("  Task: %s\n", ui.Value(taskID, ui.TaskColor))
	fmt.Println()
	return promptYN("Are you sure you want to reset this task?")
}

// promptYN displays a Y/n prompt and reads from stdin.
// Default is Y (pressing enter without input confirms).
func promptYN(message string) bool {
	fmt.Printf("%s [Y/n]: ", message)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}
