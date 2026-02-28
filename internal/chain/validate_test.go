package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validChain() *Chain {
	return &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "V0001", Name: "valid"},
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

func TestValidate_Valid(t *testing.T) {
	result := Validate(context.Background(), validChain())
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 100, result.Score)
}

func TestValidate_S1_DuplicateRefs(t *testing.T) {
	c := validChain()
	c.Festivals[1].Ref = "a" // duplicate
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S1"))
}

func TestValidate_S2_DuplicateIDs(t *testing.T) {
	c := validChain()
	c.Festivals[1].ID = "A0001" // duplicate
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S2"))
}

func TestValidate_S3_UnknownEdgeRef(t *testing.T) {
	c := validChain()
	c.Edges = append(c.Edges, Edge{From: "a", To: "z", Type: EdgeHard})
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S3"))
}

func TestValidate_S4_SelfEdge(t *testing.T) {
	c := validChain()
	c.Edges = append(c.Edges, Edge{From: "a", To: "a", Type: EdgeHard})
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S4"))
}

func TestValidate_S5_Cycle(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "CY0001", Name: "cycle"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
			{Ref: "c", ID: "C0001", Name: "gamma"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
			{From: "b", To: "c", Type: EdgeHard},
			{From: "c", To: "a", Type: EdgeHard}, // cycle
		},
	}
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S5"))
}

func TestValidate_S6_InvalidEdgeType(t *testing.T) {
	c := validChain()
	c.Edges[0].Type = "medium"
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S6"))
}

func TestValidate_S7_MissingFromWave(t *testing.T) {
	c := validChain()
	// Remove "c" from wave 3
	c.Waves[2].Festivals = []string{}
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S7"))
}

func TestValidate_S7_DuplicateInWaves(t *testing.T) {
	c := validChain()
	// Add "a" to wave 2 as well
	c.Waves[1].Festivals = append(c.Waves[1].Festivals, "a")
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S7"))
}

func TestValidate_S8_NonSequentialWaveIDs(t *testing.T) {
	c := validChain()
	c.Waves[1].ID = 5 // break sequence
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S8"))
}

func TestValidate_S8_SameWaveHardDep(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "SW0001", Name: "same-wave"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
		},
		Waves: []Wave{
			{ID: 1, Name: "Same", Unlock: "none", Festivals: []string{"a", "b"}},
		},
	}
	result := Validate(context.Background(), c)
	assert.True(t, hasErrorCode(result, "S8"))
}

func TestValidate_S9_InvalidUnlockSyntax(t *testing.T) {
	c := validChain()
	c.Waves[1].Unlock = "invalid_format"
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S9"))
}

func TestValidate_S9_UnknownRefInUnlock(t *testing.T) {
	c := validChain()
	c.Waves[1].Unlock = "z:completed"
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S9"))
}

func TestValidate_S10_TooFewFestivals(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "X0001", Name: "tiny"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
		Edges: []Edge{},
	}
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S10"))
}

func TestValidate_S10_NoEdges(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "X0001", Name: "no-edges"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []Edge{},
	}
	result := Validate(context.Background(), c)
	assert.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "S10"))
}

func TestValidate_NoWaves_SkipsWaveChecks(t *testing.T) {
	c := &Chain{
		ChainVersion: "1.0",
		Metadata:     Metadata{ID: "NW0001", Name: "no-waves"},
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Type: EdgeHard},
		},
	}
	result := Validate(context.Background(), c)
	assert.True(t, result.Valid)
	// Should not have S7, S8, S9 errors since waves are optional.
	assert.False(t, hasErrorCode(result, "S7"))
	assert.False(t, hasErrorCode(result, "S8"))
	assert.False(t, hasErrorCode(result, "S9"))
}

func TestValidate_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := Validate(ctx, validChain())
	require.False(t, result.Valid)
	assert.True(t, hasErrorCode(result, "CTX"))
}

func TestValidate_CompoundUnlock(t *testing.T) {
	c := validChain()
	c.Waves[2].Unlock = "a:completed AND b:completed"
	result := Validate(context.Background(), c)
	assert.True(t, result.Valid)
}

func hasErrorCode(r *ValidationResult, code string) bool {
	for _, e := range r.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}
