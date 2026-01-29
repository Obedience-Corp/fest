// Package scope defines command scope levels for fest CLI.
// Commands declare their required scope via Cobra Annotations,
// and the scope middleware resolves the appropriate context.
package scope

// CommandScope declares what filesystem context a command requires.
type CommandScope string

const (
	// Global commands need no filesystem context.
	// They work from any directory without resolving workspace or festival.
	// Examples: help, version, understand, intro, config, shell-init, completion
	Global CommandScope = "global"

	// Workspace commands need the festivals/ root directory resolved.
	// Resolution chain: .campaign/ detection → .workspace marker → nearest festivals/
	// Examples: list, show, status, go, create festival, validate
	Workspace CommandScope = "workspace"

	// Festival commands need a specific festival directory resolved.
	// Resolution: cwd walk-up → navigation link → --festival flag (if present)
	// Examples: progress, execute, context, create phase, deps
	Festival CommandScope = "festival"
)

// String returns the string representation of the scope.
func (s CommandScope) String() string {
	return string(s)
}
