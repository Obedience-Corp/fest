package workflow

import (
	"fmt"
	"path/filepath"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/hooks"
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

	// An Evidence block the parser could not fully read is refused before the
	// judge starts. The judge can only see what Evidence lists, so an unfilled
	// scaffold placeholder ("- (attach the task outputs for this step)") or a
	// path with a space used to be dropped in silence, leaving a list that
	// looked deliberate. The judge would then reject for missing deliverables
	// that were never handed to it, which costs a full round-trip and reads as
	// the judge being wrong rather than the gate being unfilled.
	if len(step.EvidenceUnparsed) > 0 {
		return festerrors.Validation("approval readiness failed: evidence list has entries that are not paths: "+
			strings.Join(step.EvidenceUnparsed, " | ")).
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("unreadable", strings.Join(step.EvidenceUnparsed, " | ")).
			WithHint("each **Evidence:** bullet must be a single phase-relative path with no spaces; " +
				"replace the listed lines in GATES.md with the real deliverables, then re-submit " +
				"with 'fest workflow judge'")
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
		// Name the paths in the message, not only the structured field: this
		// text is what the operator reads in the terminal and what gets
		// recorded as the step's rejection reason. "listed evidence missing or
		// empty" alone sends them back to GATES.md to work out which one.
		missing := strings.Join(status.missing, ", ")
		return festerrors.Validation("approval readiness failed: listed evidence missing or empty: " + missing).
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("missing", missing).
			WithHint("create every file listed under **Evidence:**, then re-submit with 'fest workflow judge': " + missing)
	}

	if presentationRequired && !status.presentationFound {
		return festerrors.Validation("approval readiness failed: presentation artifact missing or empty").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("required", "output_specs/PRESENTATION.md").
			WithField("missing", strings.Join(status.missing, ", ")).
			WithHint("write output_specs/PRESENTATION.md relative to the phase directory " +
				"(e.g. <phase>/output_specs/PRESENTATION.md), then re-submit with 'fest workflow judge'")
	}

	if status.found == 0 {
		return festerrors.Validation("approval readiness failed: no non-empty evidence files found").
			WithField("step", step.Number).
			WithField("step_name", step.Name).
			WithField("checked", strings.Join(paths, ", ")).
			WithHint("create at least one of these files relative to the phase directory, then " +
				"re-submit with 'fest workflow judge': " + strings.Join(paths, ", "))
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

// resolveExistingEvidencePaths returns the step's phase-relative deliverable
// paths that exist as non-empty regular files. It reuses the same path
// resolution and inspection the readiness gate applies, so the judge receives
// exactly the evidence set readiness considered present. Missing or empty
// candidates are dropped rather than surfaced; the judge is handed only files
// it can actually read. Returns nil when the step has no conventional evidence.
func resolveExistingEvidencePaths(phasePath string, step wf.WorkflowStep) []string {
	return resolveExistingEvidencePathsWithInspector(phasePath, step, inspectApprovalEvidence)
}

func resolveExistingEvidencePathsWithInspector(
	phasePath string,
	step wf.WorkflowStep,
	inspect approvalEvidenceInspector,
) []string {
	paths := wf.DefaultEvidencePaths(step)
	if len(paths) == 0 {
		return nil
	}
	var present []string
	seen := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		rel, err := normalizeEvidencePath(rawPath)
		if err != nil {
			continue
		}
		if _, dup := seen[rel]; dup {
			continue
		}
		ok, err := inspect(phasePath, rel)
		if err != nil || !ok {
			continue
		}
		seen[rel] = struct{}{}
		present = append(present, rel)
	}
	return present
}

func normalizeEvidencePath(path string) (string, error) {
	return hooks.NormalizeEvidencePath(path)
}

func inspectApprovalEvidence(phasePath, relativePath string) (bool, error) {
	return hooks.WithinRoot(phasePath, relativePath)
}

// formatReadinessBlockReason turns a readiness error into durable step feedback.
func formatReadinessBlockReason(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("approval readiness: %s", err.Error())
}
