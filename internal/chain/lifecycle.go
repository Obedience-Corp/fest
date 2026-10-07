package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// ValidTransitions returns the valid next states from the current state.
func ValidTransitions(current ChainStatus) []ChainStatus {
	switch current {
	case StatusPlanning:
		return []ChainStatus{StatusActive}
	case StatusActive:
		return []ChainStatus{StatusCompleted}
	case StatusCompleted:
		return nil
	default:
		return nil
	}
}

// Transition attempts to move a chain to the target lifecycle state.
// It validates the transition is legal and records the change in status history.
func Transition(ctx context.Context, c *Chain, target ChainStatus, notes string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	valid := ValidTransitions(c.Metadata.Status)
	allowed := false
	for _, v := range valid {
		if v == target {
			allowed = true
			break
		}
	}

	if !allowed {
		return errors.Validation("invalid chain status transition").
			WithField("from", string(c.Metadata.Status)).
			WithField("to", string(target)).
			WithField("valid", valid)
	}

	c.Metadata.Status = target
	c.Metadata.StatusHistory = append(c.Metadata.StatusHistory, StatusEntry{
		Status:    target,
		Timestamp: time.Now().UTC(),
		Notes:     notes,
	})

	return nil
}

// ForceComplete moves a planning or active chain straight to completed for
// `fest chain complete --force`, recording a single completed entry in the
// status history. The regular lifecycle in ValidTransitions is unchanged.
func ForceComplete(ctx context.Context, c *Chain, notes string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if c.Metadata.Status != StatusPlanning && c.Metadata.Status != StatusActive {
		return errors.Validation(fmt.Sprintf("cannot force-complete chain %s from status %q",
			c.Metadata.ID, c.Metadata.Status)).
			WithField("from", string(c.Metadata.Status))
	}

	c.Metadata.Status = StatusCompleted
	c.Metadata.StatusHistory = append(c.Metadata.StatusHistory, StatusEntry{
		Status:    StatusCompleted,
		Timestamp: time.Now().UTC(),
		Notes:     notes,
	})

	return nil
}
