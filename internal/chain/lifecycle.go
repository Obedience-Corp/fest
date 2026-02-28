package chain

import (
	"context"
	"fmt"
	"time"
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
		return fmt.Errorf("invalid transition from %q to %q (valid: %v)",
			c.Metadata.Status, target, valid)
	}

	c.Metadata.Status = target
	c.Metadata.StatusHistory = append(c.Metadata.StatusHistory, StatusEntry{
		Status:    target,
		Timestamp: time.Now().UTC(),
		Notes:     notes,
	})

	return nil
}
