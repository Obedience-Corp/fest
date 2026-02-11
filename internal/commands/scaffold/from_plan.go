package scaffold

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/id"
	sc "github.com/Obedience-Corp/fest/internal/scaffold"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type fromPlanOptions struct {
	PlanPath   string
	Name       string
	Dest       string // "active" or "planning"
	DryRun     bool
	JSONOutput bool
	AgentMode  bool
}

type fromPlanResult struct {
	OK               bool     `json:"ok"`
	Action           string   `json:"action"`
	FestivalDir      string   `json:"festival_dir,omitempty"`
	PhasesCreated    int      `json:"phases_created"`
	SequencesCreated int      `json:"sequences_created"`
	TasksCreated     int      `json:"tasks_created"`
	FilesCreated     []string `json:"files_created,omitempty"`
	DirsCreated      []string `json:"dirs_created,omitempty"`
	DryRun           bool     `json:"dry_run,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func newFromPlanCmd() *cobra.Command {
	opts := &fromPlanOptions{}

	cmd := &cobra.Command{
		Use:   "from-plan",
		Short: "Generate festival structure from a plan document",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Long: `Read a markdown plan document and generate the corresponding festival structure.

The plan document should follow the STRUCTURE.md format with a hierarchy section
containing phases, sequences, and tasks.

Examples:
  # Generate from a plan document
  fest scaffold from-plan --plan path/to/STRUCTURE.md --name my-festival

  # Preview what would be created
  fest scaffold from-plan --plan STRUCTURE.md --name my-fest --dry-run

  # Agent mode with JSON output
  fest scaffold from-plan --plan STRUCTURE.md --name my-fest --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.PlanPath == "" {
				return fmt.Errorf("--plan is required")
			}
			if opts.Name == "" {
				return fmt.Errorf("--name is required")
			}
			return runFromPlan(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.PlanPath, "plan", "", "Path to the plan document (required)")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Festival name (required)")
	cmd.Flags().StringVar(&opts.Dest, "dest", "active", "Destination: active or planning")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview without creating files")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "Emit JSON output")
	cmd.Flags().BoolVar(&opts.AgentMode, "agent", false, "Agent mode: JSON output")

	return cmd
}

func runFromPlan(ctx context.Context, opts *fromPlanOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if opts.AgentMode {
		opts.JSONOutput = true
	}

	// Parse plan document
	parser := sc.NewPlanParser()
	plan, err := parser.ParseFile(ctx, opts.PlanPath)
	if err != nil {
		return emitError(opts, fmt.Errorf("parsing plan document: %w", err))
	}

	// Override festival name if provided
	if opts.Name != "" {
		plan.FestivalName = opts.Name
	}

	// Find festivals root
	cwd, err := os.Getwd()
	if err != nil {
		return emitError(opts, fmt.Errorf("getting current directory: %w", err))
	}
	festivalsRoot, err := workspace.FindFestivals(cwd)
	if err != nil {
		return emitError(opts, fmt.Errorf("finding festivals directory: %w", err))
	}
	if festivalsRoot == "" {
		return emitError(opts, fmt.Errorf("no festivals directory found - run from a workspace with festivals/"))
	}

	// Determine destination
	destCategory := strings.ToLower(strings.TrimSpace(opts.Dest))
	if destCategory != "planning" && destCategory != "active" {
		destCategory = "active"
	}

	// Generate festival ID and directory name
	festivalID, err := id.GenerateID(opts.Name, festivalsRoot)
	if err != nil {
		return emitError(opts, fmt.Errorf("generating festival ID: %w", err))
	}

	slug := slugify(opts.Name)
	dirName := fmt.Sprintf("%s-%s", slug, festivalID)
	festivalDir := filepath.Join(festivalsRoot, destCategory, dirName)

	// Check if directory already exists
	if _, err := os.Stat(festivalDir); err == nil {
		return emitError(opts, fmt.Errorf("festival directory already exists: %s", festivalDir))
	}

	// Template root is inside the festivals directory
	tmplRoot := filepath.Join(festivalsRoot, ".festival", "templates")

	// Run scaffold
	runner := sc.NewRunner(sc.RunnerOptions{
		FestivalDir:  festivalDir,
		TemplateRoot: tmplRoot,
		DryRun:       opts.DryRun,
	})

	result, err := runner.Run(ctx, plan)
	if err != nil {
		return emitError(opts, err)
	}

	// Emit result
	return emitResult(opts, result, festivalID)
}

func emitResult(opts *fromPlanOptions, result *sc.RunResult, festivalID string) error {
	if opts.JSONOutput {
		r := fromPlanResult{
			OK:               true,
			Action:           "scaffold_from_plan",
			FestivalDir:      result.FestivalDir,
			PhasesCreated:    result.PhasesCreated,
			SequencesCreated: result.SequencesCreated,
			TasksCreated:     result.TasksCreated,
			FilesCreated:     result.FilesCreated,
			DirsCreated:      result.DirsCreated,
			DryRun:           opts.DryRun,
		}
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if opts.DryRun {
		fmt.Printf("%s Dry run - would create:\n\n", ui.Warning("DRY RUN"))
	} else {
		fmt.Printf("%s Festival scaffolded from plan\n\n", ui.Success("Created"))
	}

	fmt.Printf("  Festival: %s\n", ui.Accent(filepath.Base(result.FestivalDir)))
	fmt.Printf("  ID:       %s\n", festivalID)
	fmt.Printf("  Path:     %s\n\n", result.FestivalDir)

	fmt.Printf("  Phases:    %d\n", result.PhasesCreated)
	fmt.Printf("  Sequences: %d\n", result.SequencesCreated)
	fmt.Printf("  Tasks:     %d\n\n", result.TasksCreated)

	if opts.DryRun {
		fmt.Println("Directories:")
		for _, d := range result.DirsCreated {
			fmt.Printf("  %s %s\n", ui.Dim("+"), d)
		}
		fmt.Println("\nFiles:")
		for _, f := range result.FilesCreated {
			fmt.Printf("  %s %s\n", ui.Dim("+"), f)
		}
	} else {
		fmt.Println("Run " + ui.Accent("fest validate") + " to verify the structure.")
	}

	return nil
}

func emitError(opts *fromPlanOptions, err error) error {
	if opts.JSONOutput {
		r := fromPlanResult{
			OK:     false,
			Action: "scaffold_from_plan",
			Error:  err.Error(),
		}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	return err
}

// slugify converts a name to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
