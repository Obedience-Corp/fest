package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		current ChainStatus
		want    []ChainStatus
	}{
		{StatusPlanning, []ChainStatus{StatusActive}},
		{StatusActive, []ChainStatus{StatusCompleted}},
		{StatusCompleted, nil},
		{ChainStatus("unknown"), nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.current), func(t *testing.T) {
			assert.Equal(t, tt.want, ValidTransitions(tt.current))
		})
	}
}

func TestTransition_Valid(t *testing.T) {
	c := &Chain{
		Metadata: Metadata{Status: StatusPlanning},
	}

	err := Transition(context.Background(), c, StatusActive, "activating chain")
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Metadata.Status)
	require.Len(t, c.Metadata.StatusHistory, 1)
	assert.Equal(t, StatusActive, c.Metadata.StatusHistory[0].Status)
	assert.Equal(t, "activating chain", c.Metadata.StatusHistory[0].Notes)
}

func TestTransition_PlanningToCompleted_Invalid(t *testing.T) {
	c := &Chain{
		Metadata: Metadata{Status: StatusPlanning},
	}

	err := Transition(context.Background(), c, StatusCompleted, "skip")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid chain status transition")
	// Status should not change.
	assert.Equal(t, StatusPlanning, c.Metadata.Status)
}

func TestTransition_CompletedToAnything_Invalid(t *testing.T) {
	c := &Chain{
		Metadata: Metadata{Status: StatusCompleted},
	}

	err := Transition(context.Background(), c, StatusActive, "reactivate")
	assert.Error(t, err)
}

func TestTransition_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Chain{
		Metadata: Metadata{Status: StatusPlanning},
	}

	err := Transition(ctx, c, StatusActive, "cancel test")
	assert.Error(t, err)
}

func TestTransition_FullLifecycle(t *testing.T) {
	c := &Chain{
		Metadata: Metadata{Status: StatusPlanning},
	}

	require.NoError(t, Transition(context.Background(), c, StatusActive, "start"))
	assert.Equal(t, StatusActive, c.Metadata.Status)

	require.NoError(t, Transition(context.Background(), c, StatusCompleted, "done"))
	assert.Equal(t, StatusCompleted, c.Metadata.Status)

	assert.Len(t, c.Metadata.StatusHistory, 2)
}
