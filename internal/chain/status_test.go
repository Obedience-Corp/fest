package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeProgress_AllPending(t *testing.T) {
	c := makeLinearChain()
	statuses := map[string]FestivalStatus{
		"a": FestivalPlanning,
		"b": FestivalPlanning,
		"c": FestivalPlanning,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)
	assert.Equal(t, StatusPlanning, p.State)
	assert.Equal(t, 0, p.Completed)
	assert.Equal(t, 3, p.Total)
	assert.InDelta(t, 0.0, p.Percentage, 0.01)
}

func TestComputeProgress_Partial(t *testing.T) {
	c := makeLinearChain()
	statuses := map[string]FestivalStatus{
		"a": FestivalCompleted,
		"b": FestivalActive,
		"c": FestivalPlanning,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)
	assert.Equal(t, StatusActive, p.State)
	assert.Equal(t, 1, p.Completed)
	assert.InDelta(t, 33.33, p.Percentage, 0.1)
}

func TestComputeProgress_AllCompleted(t *testing.T) {
	c := makeLinearChain()
	statuses := map[string]FestivalStatus{
		"a": FestivalCompleted,
		"b": FestivalCompleted,
		"c": FestivalCompleted,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, p.State)
	assert.Equal(t, 3, p.Completed)
	assert.InDelta(t, 100.0, p.Percentage, 0.01)
}

func TestComputeProgress_WaveStatus(t *testing.T) {
	c := makeLinearChain()
	statuses := map[string]FestivalStatus{
		"a": FestivalCompleted,
		"b": FestivalActive,
		"c": FestivalPlanning,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)

	assert.Equal(t, "completed", p.WaveStatus[1].State)
	assert.Equal(t, "active", p.WaveStatus[2].State)
	assert.Equal(t, "pending", p.WaveStatus[3].State)
}

func TestComputeProgress_Unblocked(t *testing.T) {
	c := makeDiamondChain()
	statuses := map[string]FestivalStatus{
		"a": FestivalCompleted,
		"b": FestivalPlanning,
		"c": FestivalPlanning,
		"d": FestivalPlanning,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)

	// b and c should be unblocked (their only hard dep "a" is completed)
	// d should NOT be unblocked (depends on b and c)
	assert.Contains(t, p.Unblocked, "b")
	assert.Contains(t, p.Unblocked, "c")
	assert.NotContains(t, p.Unblocked, "d")
}

func TestComputeProgress_SoftDepsIgnored(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "SD0001", Name: "soft"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeSoft}, // soft dep
		},
	}
	statuses := map[string]FestivalStatus{
		"a": FestivalPlanning,
		"b": FestivalPlanning,
	}

	p, err := ComputeProgress(context.Background(), c, statuses)
	require.NoError(t, err)

	// b should be unblocked because soft deps don't block
	assert.Contains(t, p.Unblocked, "b")
}

func TestComputeProgress_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ComputeProgress(ctx, makeLinearChain(), nil)
	assert.Error(t, err)
}
