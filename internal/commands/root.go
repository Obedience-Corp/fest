package commands

import (
	"fmt"

	chaincmd "github.com/Obedience-Corp/fest/internal/commands/chain"
	commitcmd "github.com/Obedience-Corp/fest/internal/commands/commit"
	"github.com/Obedience-Corp/fest/internal/commands/config"
	contextcmd "github.com/Obedience-Corp/fest/internal/commands/context"
	depscmd "github.com/Obedience-Corp/fest/internal/commands/deps"
	"github.com/Obedience-Corp/fest/internal/commands/release"
	"github.com/Obedience-Corp/fest/internal/commands/extensions"
	feedbackcmd "github.com/Obedience-Corp/fest/internal/commands/feedback"
	"github.com/Obedience-Corp/fest/internal/commands/festival"
	"github.com/Obedience-Corp/fest/internal/commands/gates"
	idcmd "github.com/Obedience-Corp/fest/internal/commands/id"
	introcmd "github.com/Obedience-Corp/fest/internal/commands/intro"
	listcmd "github.com/Obedience-Corp/fest/internal/commands/list"
	manifestcmd "github.com/Obedience-Corp/fest/internal/commands/manifest"
	"github.com/Obedience-Corp/fest/internal/commands/markers"
	"github.com/Obedience-Corp/fest/internal/commands/migrate"
	"github.com/Obedience-Corp/fest/internal/commands/navigation"
	nextcmd "github.com/Obedience-Corp/fest/internal/commands/next"
	parsecmd "github.com/Obedience-Corp/fest/internal/commands/parse"
	progresscmd "github.com/Obedience-Corp/fest/internal/commands/progress"
	promotecmd "github.com/Obedience-Corp/fest/internal/commands/promote"
	"github.com/Obedience-Corp/fest/internal/commands/research"
	ritualcmd "github.com/Obedience-Corp/fest/internal/commands/ritual"
	scaffoldcmd "github.com/Obedience-Corp/fest/internal/commands/scaffold"
	searchcmd "github.com/Obedience-Corp/fest/internal/commands/search"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/status"
	"github.com/Obedience-Corp/fest/internal/commands/structure"
	"github.com/Obedience-Corp/fest/internal/commands/system"
	taskcmd "github.com/Obedience-Corp/fest/internal/commands/task"
	templatescmd "github.com/Obedience-Corp/fest/internal/commands/templates"
	typescmd "github.com/Obedience-Corp/fest/internal/commands/types"
	understandcmd "github.com/Obedience-Corp/fest/internal/commands/understand"
	"github.com/Obedience-Corp/fest/internal/commands/validation"
	"github.com/Obedience-Corp/fest/internal/commands/wizard"
	workflowcmd "github.com/Obedience-Corp/fest/internal/commands/workflow"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/spf13/cobra"

	// Import tui package for its init() side effects (registers hooks)
	_ "github.com/Obedience-Corp/fest/internal/commands/tui"
)

var (
	// Global flags
	configFile string
	verbose    bool
	noColor    bool
	debug      bool
)

// IsVerbose returns the global verbose flag value.
func IsVerbose() bool {
	return verbose
}

var rootCmd = &cobra.Command{
	Use:           "fest",
	Short:         "Festival Methodology CLI - goal-oriented project management for AI agents",
	Version:       fmt.Sprintf("%s (built %s, commit %s)", version.Version, version.BuildDate, version.Commit),
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Register template functions for styled help
	cobra.AddTemplateFunc("styleLabel", ui.Label)
	cobra.AddTemplateFunc("styleCategory", ui.Category)
	cobra.AddTemplateFunc("styleDim", ui.Dim)
	cobra.AddTemplateFunc("styleAccent", ui.Accent)
	cobra.AddTemplateFunc("styleBold", ui.BoldText)
	cobra.AddTemplateFunc("styleHelp", ui.StyleHelpText)

	// Set styled Long description for root command
	rootCmd.Long = styledLongDescription()

	// Apply styled help and usage templates
	rootCmd.SetHelpTemplate(styledHelpTemplate())
	rootCmd.SetUsageTemplate(styledUsageTemplate())

	// Disable the default completion command - we provide our own with better docs
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add custom completion command with better documentation
	completionCmd := system.NewCompletionCommand(rootCmd)
	completionCmd.GroupID = "system"
	rootCmd.AddCommand(completionCmd)

	// Define command groups for organized help output
	rootCmd.AddGroup(
		&cobra.Group{ID: "learning", Title: ui.Category("Learning Commands:")},
		&cobra.Group{ID: "creation", Title: ui.Category("Creation Commands:")},
		&cobra.Group{ID: "structure", Title: ui.Category("Structure Commands:")},
		&cobra.Group{ID: "workflow", Title: ui.Category("Workflow Commands:")},
		&cobra.Group{ID: "query", Title: ui.Category("Query Commands:")},
		&cobra.Group{ID: "navigation", Title: ui.Category("Navigation Commands:")},
		&cobra.Group{ID: "system", Title: ui.Category("System Commands:")},
	)

	// Enforce being inside a festivals/ tree for most commands
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Sync global flags to shared package for subpackages
		shared.SetVerbose(verbose)
		shared.SetNoColor(noColor)
		shared.SetConfigFile(configFile)
		ui.SetNoColor(noColor)

		// Initialize color palette from user's theme config
		ui.InitPalette(cmd.Context())

		// Resolve command scope using annotations.
		// Global commands work anywhere, workspace commands need festivals root,
		// festival commands need a resolved festival context.
		return scope.Resolve(cmd)
	}
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default: ~/.config/fest/config.json)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	// === LEARNING COMMANDS ===
	introCmd := introcmd.NewIntroCommand()
	introCmd.GroupID = "learning"
	rootCmd.AddCommand(introCmd)

	understandCmd := understandcmd.NewUnderstandCommand()
	understandCmd.GroupID = "learning"
	rootCmd.AddCommand(understandCmd)

	validateCmd := validation.NewValidateCommand()
	validateCmd.GroupID = "learning"
	rootCmd.AddCommand(validateCmd)

	markersCmd := markers.NewMarkersCommand()
	markersCmd.GroupID = "learning"
	rootCmd.AddCommand(markersCmd)

	// === CREATION COMMANDS ===
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create festivals, phases, sequences, or tasks (TUI)",
		GroupID: "creation",
		Annotations: map[string]string{
			"scope":         string(scope.Workspace),
			"agent_allowed": "false",
			"agent_reason":  "Launches interactive TUI picker when run without subcommand",
			"interactive":   "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return shared.StartCreateTUI(cmd.Context())
		},
	}
	createCmd.AddCommand(festival.NewCreateFestivalCommand())
	createCmd.AddCommand(festival.NewCreatePhaseCommand())
	createCmd.AddCommand(festival.NewCreateSequenceCommand())
	createCmd.AddCommand(festival.NewCreateTaskCommand())
	createCmd.AddCommand(festival.NewCreateWorkflowCommand())
	rootCmd.AddCommand(createCmd)

	scaffoldCmd := scaffoldcmd.NewScaffoldCommand()
	scaffoldCmd.GroupID = "creation"
	rootCmd.AddCommand(scaffoldCmd)

	insertCmd := structure.NewInsertCommand()
	insertCmd.GroupID = "creation"
	rootCmd.AddCommand(insertCmd)

	applyCmd := festival.NewApplyCommand()
	applyCmd.GroupID = "creation"
	rootCmd.AddCommand(applyCmd)

	// === STRUCTURE COMMANDS ===
	renumberCmd := structure.NewRenumberCommand()
	renumberCmd.GroupID = "structure"
	rootCmd.AddCommand(renumberCmd)

	reorderCmd := structure.NewReorderCommand()
	reorderCmd.GroupID = "structure"
	rootCmd.AddCommand(reorderCmd)

	removeCmd := structure.NewRemoveCommand()
	removeCmd.GroupID = "structure"
	rootCmd.AddCommand(removeCmd)

	// === NAVIGATION COMMANDS ===
	goCmd := navigation.NewGoCommand()
	goCmd.GroupID = "navigation"
	rootCmd.AddCommand(goCmd)

	linkCmd := navigation.NewLinkCommand()
	linkCmd.GroupID = "navigation"
	rootCmd.AddCommand(linkCmd)

	unlinkCmd := navigation.NewUnlinkCommand()
	unlinkCmd.GroupID = "navigation"
	rootCmd.AddCommand(unlinkCmd)

	linksCmd := navigation.NewLinksCommand()
	linksCmd.GroupID = "navigation"
	rootCmd.AddCommand(linksCmd)

	moveCmd := navigation.NewMoveCommand()
	moveCmd.GroupID = "navigation"
	rootCmd.AddCommand(moveCmd)

	// === WORKFLOW COMMANDS ===
	nextCmd := nextcmd.NewNextCommand()
	nextCmd.GroupID = "workflow"
	rootCmd.AddCommand(nextCmd)

	progressCmd := progresscmd.NewProgressCommand()
	progressCmd.GroupID = "workflow"
	rootCmd.AddCommand(progressCmd)

	workflowCmd := workflowcmd.NewWorkflowCommand()
	workflowCmd.GroupID = "workflow"
	rootCmd.AddCommand(workflowCmd)

	promoteCmd := promotecmd.NewPromoteCommand()
	promoteCmd.GroupID = "workflow"
	rootCmd.AddCommand(promoteCmd)

	ritualCmd := ritualcmd.NewRitualCommand()
	ritualCmd.GroupID = "workflow"
	rootCmd.AddCommand(ritualCmd)

	taskCmd := taskcmd.NewTaskCommand()
	taskCmd.GroupID = "workflow"
	rootCmd.AddCommand(taskCmd)

	chainCmd := chaincmd.NewChainCmd()
	chainCmd.GroupID = "workflow"
	rootCmd.AddCommand(chainCmd)

	// === QUERY COMMANDS ===
	showCmd := show.NewShowCommand()
	showCmd.GroupID = "query"
	rootCmd.AddCommand(showCmd)

	rulesCmd := show.NewRulesCommand()
	rulesCmd.GroupID = "query"
	rootCmd.AddCommand(rulesCmd)

	statusCmd := status.NewStatusCommand()
	statusCmd.GroupID = "query"
	rootCmd.AddCommand(statusCmd)

	contextCmd := contextcmd.NewContextCommand()
	contextCmd.GroupID = "query"
	rootCmd.AddCommand(contextCmd)

	depsCmd := depscmd.NewDepsCommand()
	depsCmd.GroupID = "query"
	rootCmd.AddCommand(depsCmd)

	listCmd := listcmd.NewListCommand()
	listCmd.GroupID = "query"
	rootCmd.AddCommand(listCmd)

	searchCmd := searchcmd.NewSearchCommand()
	searchCmd.GroupID = "query"
	rootCmd.AddCommand(searchCmd)

	idCmd := idcmd.NewIDCommand()
	idCmd.GroupID = "query"
	rootCmd.AddCommand(idCmd)

	release.Register(rootCmd)

	indexCmd := navigation.NewIndexCommand()
	indexCmd.GroupID = "system"
	rootCmd.AddCommand(indexCmd)

	migrateCmd := migrate.NewMigrateCommand()
	migrateCmd.GroupID = "system"
	rootCmd.AddCommand(migrateCmd)

	// === SYSTEM COMMANDS ===
	initCmd := system.NewInitCommand()
	initCmd.GroupID = "system"
	rootCmd.AddCommand(initCmd)

	systemCmd := system.NewSystemCommand()
	systemCmd.GroupID = "system"
	rootCmd.AddCommand(systemCmd)

	configCmd := config.NewConfigCommand()
	configCmd.GroupID = "system"
	rootCmd.AddCommand(configCmd)

	shellInitCmd := config.NewShellInitCommand()
	shellInitCmd.GroupID = "system"
	rootCmd.AddCommand(shellInitCmd)

	extensionCmd := extensions.NewExtensionCommand()
	extensionCmd.Hidden = true
	extensionCmd.GroupID = "system"
	rootCmd.AddCommand(extensionCmd)

	if shared.NewTUICommand != nil {
		tuiCmd := shared.NewTUICommand()
		tuiCmd.GroupID = "creation"
		rootCmd.AddCommand(tuiCmd)
	}

	// Gates policy management (part of learning/validation)
	gatesCmd := gates.NewGatesCommand()
	gatesCmd.GroupID = "learning"
	rootCmd.AddCommand(gatesCmd)

	// Research document management
	researchCmd := research.NewResearchCommand()
	researchCmd.GroupID = "creation"
	rootCmd.AddCommand(researchCmd)

	// Commit traceability commands
	commitCmd := commitcmd.NewCommitCommand()
	commitCmd.GroupID = "workflow"
	rootCmd.AddCommand(commitCmd)

	commitsCmd := commitcmd.NewCommitsCommand()
	commitsCmd.GroupID = "query"
	rootCmd.AddCommand(commitsCmd)

	// Parse command for structured output
	parseCmd := parsecmd.NewParseCommand()
	parseCmd.GroupID = "query"
	rootCmd.AddCommand(parseCmd)

	// Wizard command for guided assistance
	wizardCmd := wizard.NewWizardCommand()
	wizardCmd.GroupID = "learning"
	rootCmd.AddCommand(wizardCmd)

	// Templates command for agent-created templates
	templatesCmd := templatescmd.NewTemplatesCommand()
	templatesCmd.GroupID = "creation"
	rootCmd.AddCommand(templatesCmd)

	// Feedback command for structured feedback collection
	feedbackCmd := feedbackcmd.NewFeedbackCommand()
	feedbackCmd.GroupID = "workflow"
	rootCmd.AddCommand(feedbackCmd)

	// Types command for template type discovery
	typesCmd := typescmd.NewTypesCommand()
	typesCmd.GroupID = "query"
	rootCmd.AddCommand(typesCmd)

	// Manifest command for daemon enforcement (hidden)
	manifestCmd := manifestcmd.NewManifestCommand()
	rootCmd.AddCommand(manifestCmd)
}

// styledLongDescription returns the styled long description for the root command.
func styledLongDescription() string {
	return `fest manages Festival Methodology - a goal-oriented project management
system designed for AI agent development workflows.

` + ui.Category("GETTING STARTED (for AI agents):") + `
  ` + ui.Accent("fest understand methodology") + `    Learn core principles first
  ` + ui.Accent("fest understand structure") + `      Understand the 3-level hierarchy
  ` + ui.Accent("fest understand tasks") + `          Learn when/how to create task files
  ` + ui.Accent("fest validate") + `                  Check if a festival is properly structured

` + ui.Category("COMMON WORKFLOWS:") + `
  ` + ui.Accent("fest create festival") + `           Create a new festival (interactive TUI)
  ` + ui.Accent("fest create phase/sequence") + `     Add phases or sequences to existing festival
  ` + ui.Accent("fest validate --fix") + `            Fix common structure issues automatically
  ` + ui.Accent("fest go") + `                        Navigate to festivals directory

` + ui.Category("SYSTEM MAINTENANCE:") + `
  ` + ui.Accent("fest system sync") + `               Download latest templates from source
  ` + ui.Accent("fest system update") + `             Update .festival/ methodology files

Run '` + ui.Accent("fest understand") + `' to learn the methodology before executing tasks.`
}

// styledHelpTemplate returns a styled help template for cobra commands.
func styledHelpTemplate() string {
	return `{{if .Long}}{{styleHelp .Long}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// styledUsageTemplate returns a styled usage template for cobra commands.
func styledUsageTemplate() string {
	return `{{styleCategory "Usage:"}}
  {{.UseLine}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{styleCategory "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{styleCategory "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{styleCategory "Available Commands:"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{styleAccent (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{styleAccent (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{styleCategory "Additional Commands:"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{styleAccent (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{styleCategory "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{styleCategory "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{styleCategory "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}
