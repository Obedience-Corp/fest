package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeLinearChain() *Chain {
	return &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "T0001", Name: "test"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
			{Ref: "c", ID: "C0001", Name: "gamma"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
			{From: "b", To: "c", Type: EdgeHard},
		},
		Waves: []Wave{
			{ID: 1, Name: "First", Unlock: "none", Festivals: []string{"a"}},
			{ID: 2, Name: "Second", Unlock: "a:completed", Festivals: []string{"b"}},
			{ID: 3, Name: "Third", Unlock: "b:completed", Festivals: []string{"c"}},
		},
	}
}

func makeDiamondChain() *Chain {
	return &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "D0001", Name: "diamond"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
			{Ref: "c", ID: "C0001", Name: "gamma"},
			{Ref: "d", ID: "D0001", Name: "delta"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
			{From: "a", To: "c", Type: EdgeHard},
			{From: "b", To: "d", Type: EdgeHard},
			{From: "c", To: "d", Type: EdgeHard},
		},
	}
}

func TestBuildDAG_Linear(t *testing.T) {
	c := makeLinearChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	assert.Len(t, dag.Nodes, 3)
	assert.Equal(t, 0, dag.Nodes["a"].InDegree)
	assert.Equal(t, 1, dag.Nodes["b"].InDegree)
	assert.Equal(t, 1, dag.Nodes["c"].InDegree)
}

func TestBuildDAG_Diamond(t *testing.T) {
	c := makeDiamondChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	assert.Equal(t, 0, dag.Nodes["a"].InDegree)
	assert.Equal(t, 2, dag.Nodes["d"].InDegree) // two incoming edges
}

func TestTopologicalSort_Linear(t *testing.T) {
	c := makeLinearChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	sorted, err := dag.TopologicalSort(context.Background())
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// a must come before b, b must come before c
	idxA, idxB, idxC := indexOf(sorted, "a"), indexOf(sorted, "b"), indexOf(sorted, "c")
	assert.Less(t, idxA, idxB)
	assert.Less(t, idxB, idxC)
}

func TestTopologicalSort_Diamond(t *testing.T) {
	c := makeDiamondChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	sorted, err := dag.TopologicalSort(context.Background())
	require.NoError(t, err)
	require.Len(t, sorted, 4)

	idxA := indexOf(sorted, "a")
	idxB := indexOf(sorted, "b")
	idxC := indexOf(sorted, "c")
	idxD := indexOf(sorted, "d")
	assert.Less(t, idxA, idxB)
	assert.Less(t, idxA, idxC)
	assert.Less(t, idxB, idxD)
	assert.Less(t, idxC, idxD)
}

func TestTopologicalSort_Cycle(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "CY0001", Name: "cycle"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
			{From: "b", To: "a", Type: EdgeHard},
		},
	}
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	_, err = dag.TopologicalSort(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestDAG_Roots(t *testing.T) {
	c := makeDiamondChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	roots := dag.Roots()
	assert.Len(t, roots, 1)
	assert.Equal(t, "a", roots[0])
}

func TestDAG_DownstreamUpstream(t *testing.T) {
	c := makeDiamondChain()
	dag, err := BuildDAG(context.Background(), c)
	require.NoError(t, err)

	downstream := dag.Downstream("a")
	assert.Len(t, downstream, 2)

	upstream := dag.Upstream("d")
	assert.Len(t, upstream, 2)
}

func TestBuildDAG_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := BuildDAG(ctx, makeLinearChain())
	assert.Error(t, err)
}

func TestTopologicalSort_ContextCancellation(t *testing.T) {
	dag, err := BuildDAG(context.Background(), makeLinearChain())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = dag.TopologicalSort(ctx)
	assert.Error(t, err)
}

func indexOf(slice []string, val string) int {
	for i, s := range slice {
		if s == val {
			return i
		}
	}
	return -1
}
