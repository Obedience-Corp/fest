// Package guidance provides the command registry for mode-specific CLI commands.
package guidance

// ModeCommands defines the command strings for a specific guidance mode.
// Each mode has its own set of commands for starting, navigating, and completing work.
// This centralizes command strings that were previously hardcoded in navigators.
type ModeCommands struct {
	// DisplayName is the human-readable mode name (e.g., "Implementation Mode")
	DisplayName string

	// StartCommand is the command to start/enter this mode
	StartCommand string

	// NextCommand is the command to get the next step/item
	NextCommand string

	// CompleteCommand is the base command to mark work complete.
	// Note: Callers typically append task paths or item IDs to this.
	CompleteCommand string

	// AdditionalCmds contains mode-specific commands beyond the core three.
	// Keys: "pass", "fail", "skip", "approve", "reject", etc.
	AdditionalCmds map[string]string
}

// DefaultModeCommands provides the standard command strings for each guidance mode.
// Navigators should use this registry instead of hardcoding command strings.
// This enables consistent command patterns across all modes and simplifies updates.
var DefaultModeCommands = map[Mode]ModeCommands{
	ModeImplementation: {
		DisplayName:     "Implementation Mode",
		StartCommand:    "fest execute",
		NextCommand:     "fest next",
		CompleteCommand: "fest task completed",
		AdditionalCmds:  nil,
	},
	ModePlan: {
		DisplayName:     "Planning Mode",
		StartCommand:    "fest execute --mode plan",
		NextCommand:     "fest next",
		CompleteCommand: "fest workflow advance",
		AdditionalCmds: map[string]string{
			"status":  "fest workflow status",
			"advance": "fest workflow advance",
			"approve": "fest workflow approve",
			"reject":  "fest workflow reject --reason",
			"show":    "fest workflow show",
		},
	},
	ModeResearch: {
		DisplayName:     "Research Mode",
		StartCommand:    "fest execute --mode research",
		NextCommand:     "fest next",
		CompleteCommand: "fest workflow advance",
		AdditionalCmds: map[string]string{
			"status":  "fest workflow status",
			"advance": "fest workflow advance",
			"approve": "fest workflow approve",
			"reject":  "fest workflow reject --reason",
			"show":    "fest workflow show",
		},
	},
	ModeReview: {
		DisplayName:     "Review Mode",
		StartCommand:    "fest execute --mode review",
		NextCommand:     "fest next --mode review",
		CompleteCommand: "fest task completed",
		AdditionalCmds: map[string]string{
			"pass": "fest review --pass",
			"fail": "fest review --fail",
			"skip": "fest review --skip",
		},
	},
	ModeAction: {
		DisplayName:     "Action Mode",
		StartCommand:    "fest execute --mode action",
		NextCommand:     "fest next --mode action",
		CompleteCommand: "fest task completed",
		AdditionalCmds: map[string]string{
			"complete": "fest action --complete",
			"fail":     "fest action --fail",
			"skip":     "fest action --skip",
		},
	},
	ModeIngest: {
		DisplayName:     "Ingest Mode",
		StartCommand:    "fest execute --mode ingest",
		NextCommand:     "fest next",
		CompleteCommand: "fest workflow advance",
		AdditionalCmds: map[string]string{
			"status":  "fest workflow status",
			"advance": "fest workflow advance",
			"approve": "fest workflow approve",
			"reject":  "fest workflow reject --reason",
			"show":    "fest workflow show",
		},
	},
	ModeWorkflow: {
		DisplayName:     "Workflow Mode",
		StartCommand:    "fest next",
		NextCommand:     "fest next",
		CompleteCommand: "fest workflow advance",
		AdditionalCmds: map[string]string{
			"status":  "fest workflow status",
			"advance": "fest workflow advance",
			"approve": "fest workflow approve",
			"reject":  "fest workflow reject --reason",
			"show":    "fest workflow show",
		},
	},
}

// GetModeCommands returns the command configuration for a given mode.
// Returns the implementation mode commands if the mode is not found.
func GetModeCommands(mode Mode) ModeCommands {
	if cmds, ok := DefaultModeCommands[mode]; ok {
		return cmds
	}
	return DefaultModeCommands[ModeImplementation]
}

// FormatProgressCommand returns the full command for completing a task.
// This centralizes the command format: fest task completed <path>
func FormatProgressCommand(taskPath string) string {
	return "fest task completed " + taskPath
}
