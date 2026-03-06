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

Quality gates are validation steps that run at the end of implementation sequences.
Gates are configured in fest.yaml under the implementation section.

Available Commands:
  show      Show effective gate policy from fest.yaml
  apply     Apply quality gates to sequences
  remove    Remove quality gate files from sequences`,
	}

	cmd.AddCommand(newGatesShowCmd())
	cmd.AddCommand(newGatesApplyCmd())
	cmd.AddCommand(newGatesRemoveCmd())

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

	// Try to get festivals root (may fail from linked project, that's ok)
	festivalsRoot, _ := tpl.FindFestivalsRoot(cwd)

	// Resolve paths (handles linked festivals via shared.ResolveFestivalPath)
	festivalPath, phasePath, sequencePath, err := resolvePaths(festivalsRoot, cwd, phase, sequence)
	if err != nil {
		return errors.Wrap(err, "resolving paths").WithOp("runGatesShow")
	}

	// Derive festivalsRoot from festivalPath if needed (for linked festivals)
	if festivalsRoot == "" {
		festivalsRoot = filepath.Dir(filepath.Dir(festivalPath))
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
	_, _ = fmt.Fprintln(out, ui.H1("Gate Policy"))
	_, _ = fmt.Fprintf(out, "%s %s\n", ui.Label("Scope"), ui.Value(location))
	_, _ = fmt.Fprintln(out, ui.Dim(strings.Repeat("─", 60)))

	// Show configuration sources
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, ui.H2("Configuration Sources"))
	if len(merged.Sources) == 0 {
		_, _ = fmt.Fprintln(out, ui.Dim("No configuration sources found."))
	} else {
		for _, src := range merged.Sources {
			target := src.Path
			if target == "" {
				target = src.Name
			}
			_, _ = fmt.Fprintf(out, "  %s\n", ui.Dim(target))
		}
	}

	// Active gates organized by phase type
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, ui.H2("Active Gates"))

	activeGates := merged.GetActiveGates()
	if len(activeGates) == 0 {
		_, _ = fmt.Fprintln(out, ui.Dim("No active gates."))
	} else {
		// Group gates by phase type from template path (e.g., gates/implementation/...)
		phaseGates := make(map[string][]gatescore.GateTask)
		phaseOrder := []string{"implementation"}

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
			_, _ = fmt.Fprintf(out, "\n%s\n", ui.Value(phaseType))
			for _, gate := range gates {
				_, _ = fmt.Fprintf(out, "  %s\n", ui.Value(gate.ID, ui.GateColor))
			}
		}
	}

	// Show removed gates if any
	hasRemoved := false
	for _, gate := range merged.Gates {
		if gate.Removed {
			if !hasRemoved {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, ui.H2("Removed Gates"))
				hasRemoved = true
			}
			source := "unknown"
			if gate.Source != nil {
				source = string(gate.Source.Level)
			}
			_, _ = fmt.Fprintf(out, "%s %s\n",
				ui.Warning(gate.ID),
				ui.Dim(fmt.Sprintf("removed at %s level", source)))
		}
	}

	// Show exclude patterns if any
	if len(merged.ExcludePatterns) > 0 {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, ui.H2("Exclude Patterns"))
		for _, pattern := range merged.ExcludePatterns {
			_, _ = fmt.Fprintf(out, "%s %s\n", ui.Dim("•"), ui.Dim(pattern))
		}
	}

	_, _ = fmt.Fprintln(out)
	return nil
}

// Apply command in apply.go

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
