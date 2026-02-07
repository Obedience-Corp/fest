package task

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/ui"
)

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
