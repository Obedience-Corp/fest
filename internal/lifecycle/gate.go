package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// EnforceOptions controls which festival phase the gate evaluates.
type EnforceOptions struct {
	// PhasePath, if set, is used directly as the phase to evaluate.
	PhasePath string
	// TaskID, if set, derives PhasePath from the task's location.
	// Takes precedence over the FindFirstIncompletePhase fallback.
	TaskID string
	// Reason is appended to the printed block message so the agent
	// knows which command triggered the block.
	Reason string
}

// EnforcePreActive returns an error wrapping errors.ErrAlreadyPrinted
// when the festival's current status forbids work on the resolved phase.
//
// Resolution order for the phase to evaluate:
//  1. opts.PhasePath if non-empty.
//  2. opts.TaskID (resolved via festivalPath + first path segment).
//  3. FindFirstIncompletePhase (festival root scan).
//
// Fail-closed: if the festival config or phase scan cannot be determined,
// the gate blocks rather than silently allowing the operation.
func EnforcePreActive(ctx context.Context, festivalPath string, opts EnforceOptions) error {
	festCfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return emitFailClosed(festivalPath, opts.Reason,
			"could not read fest.yaml; run 'fest validate'")
	}
	// LoadFestivalConfig returns a default config (no error) when fest.yaml
	// is missing. That leaves status undeterminable — fail closed.
	if !festCfg.Metadata.HasMetadata() || festCfg.Metadata.CurrentStatus() == "" {
		return emitFailClosed(festivalPath, opts.Reason,
			"festival metadata missing or incomplete; run 'fest validate'")
	}

	status := festCfg.Metadata.CurrentStatus()
	if status != "planning" && status != "ready" {
		return nil
	}

	phasePath := opts.PhasePath
	if phasePath == "" && opts.TaskID != "" {
		phasePath = phasePathFromTaskID(festivalPath, opts.TaskID)
	}
	if phasePath == "" {
		found, _, findErr := FindFirstIncompletePhase(ctx, festivalPath)
		if findErr != nil {
			return emitFailClosed(festivalPath, opts.Reason,
				"could not scan festival phases; run 'fest validate'")
		}
		if found == "" {
			return nil
		}
		phasePath = found
	}

	phaseType := guidance.DetectPhaseType(phasePath)
	if phaseType != "implementation" && phaseType != "review" {
		return nil
	}

	return emitBlock(festivalPath, status, opts.Reason)
}

func phasePathFromTaskID(festivalPath, taskID string) string {
	parts := strings.SplitN(taskID, "/", 2)
	if parts[0] == "" {
		return ""
	}
	return filepath.Join(festivalPath, parts[0])
}

func emitBlock(festivalPath, status, reason string) error {
	festName := filepath.Base(festivalPath)

	var sb strings.Builder
	sb.WriteString("STOP — FESTIVAL NOT YET ACTIVE\n")
	sb.WriteString("──────────────────────────────\n")

	if status == "ready" {
		fmt.Fprintf(&sb, "Festival %q is in ready status.\n", festName)
		sb.WriteString("Lifecycle: planning -> ready -> [active] -> completed\n\n")
		sb.WriteString("Before executing, confirm:\n")
		sb.WriteString("  - Is this the correct festival you should be working on?\n")
		sb.WriteString("  - Did the user approve implementation of this festival?\n\n")
	} else {
		fmt.Fprintf(&sb, "Festival %q is in planning status.\n", festName)
		sb.WriteString("Lifecycle: [planning] -> ready -> active -> completed\n\n")
		sb.WriteString("Implementation phases require active status.\n")
	}

	if reason != "" {
		fmt.Fprintf(&sb, "Action %q requires the festival to be active.\n", reason)
	}
	sb.WriteString("Promote first: fest promote\n")

	fmt.Print(sb.String())
	return errors.ErrAlreadyPrinted
}

func emitFailClosed(festivalPath, reason, hint string) error {
	festName := filepath.Base(festivalPath)

	var sb strings.Builder
	sb.WriteString("STOP — FESTIVAL STATUS UNDETERMINED\n")
	sb.WriteString("───────────────────────────────────\n")
	fmt.Fprintf(&sb, "Could not determine status for festival %q.\n", festName)
	fmt.Fprintf(&sb, "Hint: %s\n", hint)
	if reason != "" {
		fmt.Fprintf(&sb, "Action %q is blocked until status can be verified.\n", reason)
	}
	sb.WriteString("\nIf this festival was promoted but a tool failed to read its status,\n")
	sb.WriteString("run 'fest validate' to inspect, then retry.\n")
	fmt.Print(sb.String())
	return errors.ErrAlreadyPrinted
}

// Gate is the interface progress.Manager depends on for defense-in-depth
// enforcement. It mirrors progress.Gate; lifecycle.NewGate returns a value
// that satisfies progress.Gate.
type Gate interface {
	EnforceForTask(ctx context.Context, taskID string) error
}

type realGate struct {
	festivalPath string
	reason       string
}

// NewGate returns a Gate bound to the given festival. It satisfies
// progress.Gate via the EnforceForTask method.
func NewGate(festivalPath string) progress.Gate {
	return &realGate{festivalPath: festivalPath, reason: "progress.Manager"}
}

// NewGateWithReason returns a Gate that surfaces a specific command name in
// block messages. Useful for CLI sites that want the reason in the output.
func NewGateWithReason(festivalPath, reason string) progress.Gate {
	return &realGate{festivalPath: festivalPath, reason: reason}
}

func (g *realGate) EnforceForTask(ctx context.Context, taskID string) error {
	return EnforcePreActive(ctx, g.festivalPath, EnforceOptions{
		TaskID: taskID,
		Reason: g.reason,
	})
}
