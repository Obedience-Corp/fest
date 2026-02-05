package gates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newGatesInitCmd() *cobra.Command {
	var phase, sequence string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a gate configuration file",
		Long: `Create a template configuration file at the specified level.

At festival level, creates fest.yaml with quality gate settings.
At phase/sequence level, creates .fest.gates.yml override file.`,
		Example: `  fest gates init
  fest gates init --phase 002_IMPLEMENT
  fest gates init --sequence 002_IMPLEMENT/01_core`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatesInit(cmd.Context(), cmd, phase, sequence)
		},
	}

	cmd.Flags().StringVar(&phase, "phase", "", "Initialize for specific phase")
	cmd.Flags().StringVar(&sequence, "sequence", "", "Initialize for specific sequence (format: phase/sequence)")

	return cmd
}

func runGatesInit(ctx context.Context, cmd *cobra.Command, phase, sequence string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runGatesInit")
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
		return errors.Wrap(err, "resolving paths").WithOp("runGatesInit")
	}

	// Determine what to create
	if sequencePath != "" || phasePath != "" {
		// Phase or sequence level: create .fest.gates.yml
		return createPhaseOverrideFile(cmd, festivalPath, phasePath, sequencePath)
	}

	// Festival level: create fest.yaml
	return createFestYAMLFile(cmd, festivalPath)
}

func createPhaseOverrideFile(cmd *cobra.Command, festivalPath, phasePath, sequencePath string) error {
	targetPath, overrideFile := resolveTargetPath(festivalPath, phasePath, sequencePath)
	overridePath := filepath.Join(targetPath, overrideFile)

	// Check if file already exists
	if _, err := os.Stat(overridePath); err == nil {
		return errors.Validation("override file already exists").WithField("path", overridePath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(overridePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return errors.IO("creating directory", err).WithField("path", parentDir)
	}

	template := `# Gate policy override file
# See: fest understand gates

version: 1
inherit: true  # Set to false to not inherit from parent levels

# Add gates (insert after inherited gates)
# append:
#   - id: security_audit
#     template: SECURITY_AUDIT
#     enabled: true

# Exclude patterns for this level
# exclude_patterns:
#   - "*_docs"
`

	if err := os.WriteFile(overridePath, []byte(template), 0644); err != nil {
		return errors.IO("writing override file", err).WithField("path", overridePath)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, ui.Success("✓ Override file created"))
	fmt.Fprintf(out, "%s %s\n", ui.Label("Path"), ui.Dim(overridePath))
	return nil
}

func createFestYAMLFile(cmd *cobra.Command, festivalPath string) error {
	festYAMLPath := filepath.Join(festivalPath, "fest.yaml")

	// Check if file already exists
	if _, err := os.Stat(festYAMLPath); err == nil {
		return errors.Validation("fest.yaml already exists").WithField("path", festYAMLPath)
	}

	template := `# Festival Configuration
# See: fest understand config

version: "1.0"

quality_gates:
  enabled: true
  auto_append: true

  # Implementation phase gates (code changes)
  implementation:
    - id: testing
      template: gates/implementation/QUALITY_GATE_TESTING
      name: Testing and Verification
      enabled: true
    - id: review
      template: gates/implementation/QUALITY_GATE_REVIEW
      name: Code Review
      enabled: true
    - id: iterate
      template: gates/implementation/QUALITY_GATE_ITERATE
      name: Review Results and Iterate
      enabled: true
    - id: commit
      template: gates/implementation/QUALITY_GATE_COMMIT
      name: Commit Changes
      enabled: true

  # Planning phase gates
  planning:
    - id: plan_review
      template: gates/planning/QUALITY_GATE_PLAN_REVIEW
      name: Planning Review
      enabled: true
    - id: approval
      template: gates/planning/QUALITY_GATE_APPROVAL
      name: Planning Approval
      enabled: true

  # Research phase gates
  research:
    - id: findings_review
      template: gates/research/QUALITY_GATE_FINDINGS_REVIEW
      name: Findings Review
      enabled: true
    - id: documentation
      template: gates/research/QUALITY_GATE_DOCUMENTATION
      name: Documentation
      enabled: true

  # Review/QA phase gates
  review:
    - id: checklist
      template: gates/review/QUALITY_GATE_CHECKLIST
      name: Review Checklist
      enabled: true
    - id: sign_off
      template: gates/review/QUALITY_GATE_SIGN_OFF
      name: Sign-off
      enabled: true

  # Non-coding action phase gates (deployment, config, etc.)
  non_coding_action:
    - id: action_verify
      template: gates/non_coding_action/QUALITY_GATE_ACTION_VERIFY
      name: Execution and Verify
      enabled: true
    - id: completion
      template: gates/non_coding_action/QUALITY_GATE_COMPLETION
      name: Completion
      enabled: true

excluded_patterns:
  - "*_planning"
  - "*_research"
  - "*_requirements"

templates:
  task_default: tasks/SIMPLE
  prefer_simple: true

tracking:
  enabled: true
  checksum_file: .festival-checksums.json
`

	if err := os.WriteFile(festYAMLPath, []byte(template), 0644); err != nil {
		return errors.IO("writing fest.yaml", err).WithField("path", festYAMLPath)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, ui.Success("✓ Festival configuration created"))
	fmt.Fprintf(out, "%s %s\n", ui.Label("Path"), ui.Dim(festYAMLPath))
	return nil
}
