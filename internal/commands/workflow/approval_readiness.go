package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// checkAutoJudgeAllowed refuses --auto for operator_attestation checkpoints.
func checkAutoJudgeAllowed(step wf.WorkflowStep) error {
	class := wf.ClassifyCheckpoint(step)
	if class != wf.CheckpointClassOperatorAttestation {
		return nil
	}
	return festerrors.Validation("checkpoint class operator_attestation cannot be auto-judged").
		WithField("step", step.Number).
		WithField("step_name", step.Name).
		WithField("checkpoint_class", class.String()).
		WithHint("this step requires a human operator approval; run 'fest workflow approve' interactively from a terminal")
}

// checkApprovalReadiness performs deterministic pre-model checks for --auto.
// On failure the judge command must not be started.
func checkApprovalReadiness(phasePath string, step wf.WorkflowStep) error {
	if err := checkAutoJudgeAllowed(step); err != nil {
		return err
	}

	// Presentation-like steps require a non-empty presentation (or listed evidence).
	if !wf.IsPresentationStep(step) && len(step.EvidencePaths) == 0 {
		return nil
	}

	paths := wf.DefaultEvidencePaths(step)
	if len(paths) == 0 {
		return festerrors.Validation("approval readiness failed: no evidence paths configured for this step").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithHint("add an **Evidence:** list under the step, or create the conventional output_specs files")
	}

	var missing []string
	var found int
	presentationRequired := wf.IsPresentationStep(step)
	presentationFound := false

	for _, rel := range paths {
		rel = filepath.Clean(rel)
		if rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		full := filepath.Join(phasePath, rel)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() || info.Size() == 0 {
			missing = append(missing, rel)
			continue
		}
		found++
		base := filepath.Base(rel)
		if strings.EqualFold(base, "PRESENTATION.md") {
			presentationFound = true
		}
	}

	if presentationRequired && !presentationFound {
		return festerrors.Validation("approval readiness failed: presentation artifact missing or empty").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("required", "output_specs/PRESENTATION.md").
			WithField("missing", strings.Join(missing, ", ")).
			WithHint("write the presentation deliverable, then re-submit with 'fest workflow approve --auto'")
	}

	if found == 0 {
		return festerrors.Validation("approval readiness failed: no non-empty evidence files found").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("checked", strings.Join(paths, ", ")).
			WithHint("create the evidence files listed for this step, then re-submit with 'fest workflow approve --auto'")
	}

	return nil
}

// formatReadinessBlockReason turns a readiness error into durable step feedback.
func formatReadinessBlockReason(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("approval readiness: %s", err.Error())
}
