package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

type approvalEvidenceInspector func(phasePath, relativePath string) (bool, error)

type approvalEvidenceStatus struct {
	missing           []string
	found             int
	presentationFound bool
}

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
	return checkApprovalReadinessWithInspector(phasePath, step, inspectApprovalEvidence)
}

func checkApprovalReadinessWithInspector(
	phasePath string,
	step wf.WorkflowStep,
	inspect approvalEvidenceInspector,
) error {
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

	explicitEvidence := len(step.EvidencePaths) > 0
	presentationRequired := wf.IsPresentationStep(step) && !explicitEvidence
	status, err := inspectApprovalEvidencePaths(phasePath, paths, inspect)
	if err != nil {
		return festerrors.Wrap(err, "approval readiness failed").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithHint("evidence paths must be relative, non-empty files contained by the phase directory")
	}

	if explicitEvidence && len(status.missing) > 0 {
		return festerrors.Validation("approval readiness failed: listed evidence missing or empty").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("missing", strings.Join(status.missing, ", ")).
			WithHint("create every file listed under **Evidence:**, then re-submit with 'fest workflow judge'")
	}

	if presentationRequired && !status.presentationFound {
		return festerrors.Validation("approval readiness failed: presentation artifact missing or empty").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("required", "output_specs/PRESENTATION.md").
			WithField("missing", strings.Join(status.missing, ", ")).
			WithHint("write the presentation deliverable, then re-submit with 'fest workflow judge'")
	}

	if status.found == 0 {
		return festerrors.Validation("approval readiness failed: no non-empty evidence files found").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("checked", strings.Join(paths, ", ")).
			WithHint("create the evidence files listed for this step, then re-submit with 'fest workflow judge'")
	}

	return nil
}

func inspectApprovalEvidencePaths(
	phasePath string,
	paths []string,
	inspect approvalEvidenceInspector,
) (approvalEvidenceStatus, error) {
	var status approvalEvidenceStatus
	for _, rawPath := range paths {
		rel, err := normalizeEvidencePath(rawPath)
		if err != nil {
			return status, fmt.Errorf("invalid evidence path %q: %w", rawPath, err)
		}
		present, err := inspect(phasePath, rel)
		if err != nil {
			return status, fmt.Errorf("checking evidence path %q: %w", rel, err)
		}
		if !present {
			status.missing = append(status.missing, rel)
			continue
		}
		status.found++
		if strings.EqualFold(filepath.Base(rel), "PRESENTATION.md") {
			status.presentationFound = true
		}
	}
	return status, nil
}

func normalizeEvidencePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the phase directory")
	}
	return clean, nil
}

func inspectApprovalEvidence(phasePath, relativePath string) (bool, error) {
	phaseRoot, err := filepath.Abs(phasePath)
	if err != nil {
		return false, fmt.Errorf("resolving phase root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(phaseRoot)
	if err != nil {
		return false, fmt.Errorf("resolving phase root symlinks: %w", err)
	}

	candidate := filepath.Join(phaseRoot, relativePath)
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("resolving evidence symlinks: %w", err)
	}
	containedPath, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return false, fmt.Errorf("checking resolved evidence containment: %w", err)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("resolved evidence path escapes the phase directory")
	}

	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular() && info.Size() > 0, nil
}

// formatReadinessBlockReason turns a readiness error into durable step feedback.
func formatReadinessBlockReason(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("approval readiness: %s", err.Error())
}
