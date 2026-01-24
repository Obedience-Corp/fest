package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	gatescore "github.com/Obedience-Corp/fest/internal/gates"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// NewGatesCommand creates the gates command group
func NewGatesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gates",
		Short: "Manage quality gates - validation steps at sequence end",
		Long: `Manage quality gate policies for festivals.

Quality gates are validation steps that run at the end of sequences.
Gates are configured in fest.yaml by phase type (implementation, planning,
research, review, non_coding_action).

Available Commands:
  show      Show effective gate policy from fest.yaml
  apply     Apply quality gates to sequences
  remove    Remove quality gate files from sequences
  init      Initialize a fest.yaml gate configuration
  validate  Validate gate configuration`,
	}

	cmd.AddCommand(newGatesShowCmd())
	cmd.AddCommand(newGatesApplyCmd())
	cmd.AddCommand(newGatesRemoveCmd())
	cmd.AddCommand(newGatesInitCmd())
	cmd.AddCommand(newGatesValidateCmd())

	return cmd
}

// --- SHOW COMMAND ---

func newGatesShowCmd() *cobra.Command {
	var phase, sequence string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective gate policy",
		Long: `Display the effective gate policy for a festival, phase, or sequence.
Shows which gates are active and where each gate originated from.`,
		Example: `  fest gates show
  fest gates show --phase 002_IMPLEMENT
  fest gates show --sequence 002_IMPLEMENT/01_core --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatesShow(cmd.Context(), cmd, phase, sequence, jsonOutput)
		},
	}

	cmd.Flags().StringVar(&phase, "phase", "", "Show gates for specific phase")
	cmd.Flags().StringVar(&sequence, "sequence", "", "Show gates for specific sequence (format: phase/sequence)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func runGatesShow(ctx context.Context, cmd *cobra.Command, phase, sequence string, jsonOutput bool) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runGatesShow")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting working directory", err)
	}

	festivalsRoot, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals root").WithOp("runGatesShow")
	}

	festivalPath, phasePath, sequencePath, err := resolvePaths(festivalsRoot, cwd, phase, sequence)
	if err != nil {
		return errors.Wrap(err, "resolving paths").WithOp("runGatesShow")
	}

	// Use ConfigMerger with nil registry (registry is no longer needed)
	merger, err := gatescore.NewConfigMerger(festivalsRoot, nil)
	if err != nil {
		return errors.Wrap(err, "creating config merger").WithOp("runGatesShow")
	}

	opts := gatescore.DefaultMergeOptions()
	var merged *gatescore.MergedPolicy
	if sequencePath != "" {
		merged, err = merger.MergeForSequence(ctx, festivalPath, phasePath, sequencePath, opts)
	} else if phasePath != "" {
		merged, err = merger.MergeForPhase(ctx, festivalPath, phasePath, opts)
	} else {
		merged, err = merger.MergeForFestival(ctx, festivalPath, opts)
	}
	if err != nil {
		return errors.Wrap(err, "loading merged policy").WithOp("runGatesShow")
	}

	if jsonOutput {
		return printGatesShowMergedJSON(cmd, merged)
	}
	return printGatesShowMergedTable(cmd, merged, phase, sequence)
}

func printGatesShowMergedJSON(cmd *cobra.Command, merged *gatescore.MergedPolicy) error {
	output := struct {
		Gates           []gateOutput   `json:"gates"`
		Sources         []sourceOutput `json:"sources"`
		Level           string         `json:"level"`
		FestYAMLEnabled bool           `json:"fest_yaml_enabled"`
		ExcludePatterns []string       `json:"exclude_patterns,omitempty"`
	}{
		Gates:           make([]gateOutput, 0, len(merged.Gates)),
		Sources:         make([]sourceOutput, 0, len(merged.Sources)),
		Level:           string(merged.Level),
		FestYAMLEnabled: merged.FestYAMLEnabled,
		ExcludePatterns: merged.ExcludePatterns,
	}

	for _, gate := range merged.Gates {
		g := gateOutput{
			ID:       gate.ID,
			Template: gate.Template,
			Name:     gate.Name,
			Enabled:  gate.Enabled,
			Removed:  gate.Removed,
		}
		if gate.Source != nil {
			g.Source = string(gate.Source.Level)
		}
		output.Gates = append(output.Gates, g)
	}

	for _, src := range merged.Sources {
		output.Sources = append(output.Sources, sourceOutput{
			Level: string(src.Level),
			Path:  src.Path,
			Name:  src.Name,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func printGatesShowMergedTable(cmd *cobra.Command, merged *gatescore.MergedPolicy, phase, sequence string) error {
	out := cmd.OutOrStdout()

	// Header
	location := "festival"
	if sequence != "" {
		location = sequence
	} else if phase != "" {
		location = phase
	}
	fmt.Fprintln(out, ui.H1("Gate Policy"))
	fmt.Fprintf(out, "%s %s\n", ui.Label("Scope"), ui.Value(location))
	fmt.Fprintln(out, ui.Dim(strings.Repeat("─", 60)))

	// Show configuration sources
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.H2("Configuration Sources"))
	if len(merged.Sources) == 0 {
		fmt.Fprintln(out, ui.Dim("No configuration sources found."))
	} else {
		for _, src := range merged.Sources {
			target := src.Path
			if target == "" {
				target = src.Name
			}
			fmt.Fprintf(out, "  %s\n", ui.Dim(target))
		}
	}

	// Active gates organized by phase type
	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.H2("Active Gates"))

	activeGates := merged.GetActiveGates()
	if len(activeGates) == 0 {
		fmt.Fprintln(out, ui.Dim("No active gates."))
	} else {
		// Group gates by phase type from template path (e.g., gates/implementation/...)
		phaseGates := make(map[string][]gatescore.GateTask)
		phaseOrder := []string{"implementation", "planning", "research", "review", "non_coding_action"}

		for _, gate := range activeGates {
			phaseType := extractPhaseFromTemplate(gate.Template)
			phaseGates[phaseType] = append(phaseGates[phaseType], gate)
		}

		// Display gates grouped by phase type
		for _, phaseType := range phaseOrder {
			gates := phaseGates[phaseType]
			if len(gates) == 0 {
				continue
			}
			fmt.Fprintf(out, "\n%s\n", ui.Value(phaseType))
			for _, gate := range gates {
				fmt.Fprintf(out, "  %s\n", ui.Value(gate.ID, ui.GateColor))
			}
		}
	}

	// Show removed gates if any
	hasRemoved := false
	for _, gate := range merged.Gates {
		if gate.Removed {
			if !hasRemoved {
				fmt.Fprintln(out)
				fmt.Fprintln(out, ui.H2("Removed Gates"))
				hasRemoved = true
			}
			source := "unknown"
			if gate.Source != nil {
				source = string(gate.Source.Level)
			}
			fmt.Fprintf(out, "%s %s\n",
				ui.Warning(gate.ID),
				ui.Dim(fmt.Sprintf("removed at %s level", source)))
		}
	}

	// Show exclude patterns if any
	if len(merged.ExcludePatterns) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, ui.H2("Exclude Patterns"))
		for _, pattern := range merged.ExcludePatterns {
			fmt.Fprintf(out, "%s %s\n", ui.Dim("•"), ui.Dim(pattern))
		}
	}

	fmt.Fprintln(out)
	return nil
}

// Apply and init commands in apply.go and init.go

// --- VALIDATE COMMAND ---

func newGatesValidateCmd() *cobra.Command {
	var fix bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate gate configuration",
		Long:  `Check gate configuration files for errors and inconsistencies.`,
		Example: `  fest gates validate
  fest gates validate --fix
  fest gates validate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatesValidate(cmd.Context(), cmd, fix, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to fix issues automatically")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func runGatesValidate(ctx context.Context, cmd *cobra.Command, fix, jsonOutput bool) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runGatesValidate")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting working directory", err)
	}

	festivalsRoot, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals root").WithOp("runGatesValidate")
	}

	var issues []validationIssue

	// Check for override files and validate them
	err = filepath.Walk(festivalsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		if info.Name() == gatescore.PhaseOverrideFileName {
			if _, loadErr := gatescore.LoadPolicy(path); loadErr != nil {
				issues = append(issues, validationIssue{
					Path:     path,
					Severity: "error",
					Message:  fmt.Sprintf("Invalid gate override: %v", loadErr),
				})
			}
		}

		return nil
	})

	if err != nil {
		return errors.IO("walking directory", err).WithField("path", festivalsRoot)
	}

	if jsonOutput {
		output := struct {
			Valid  bool              `json:"valid"`
			Issues []validationIssue `json:"issues"`
		}{
			Valid:  len(issues) == 0,
			Issues: issues,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	out := cmd.OutOrStdout()
	if len(issues) == 0 {
		fmt.Fprintln(out, ui.Success("✓ Gate configuration is valid."))
		return nil
	}

	fmt.Fprintln(out, ui.H1("Gate Validation"))
	fmt.Fprintf(out, "%s %s\n", ui.Label("Issues"), ui.Value(fmt.Sprintf("%d", len(issues))))
	for _, issue := range issues {
		severity := strings.ToUpper(issue.Severity)
		severityLabel := ui.Warning(severity)
		if strings.EqualFold(issue.Severity, "error") {
			severityLabel = ui.Error(severity)
		}
		fmt.Fprintf(out, "\n%s %s\n", severityLabel, ui.Dim(issue.Path))
		fmt.Fprintf(out, "  %s\n", issue.Message)
	}

	return nil
}

// Note: helper types and functions moved to gates_helpers.go

// extractPhaseFromTemplate extracts the phase type from a template path.
// e.g., "gates/implementation/QUALITY_GATE_TESTING" -> "implementation"
func extractPhaseFromTemplate(template string) string {
	// Expected format: gates/<phase_type>/<gate_name>
	if strings.HasPrefix(template, "gates/") {
		parts := strings.Split(template, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return "other"
}
