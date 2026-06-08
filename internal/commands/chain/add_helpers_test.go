package chain

import (
	"testing"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugifyRef(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"spaces", "Festival App Onboarding", "festival_app_onboarding"},
		{"hyphens", "festival-app-onboarding-parity", "festival_app_onboarding_parity"},
		{"mixed punctuation", "Foo: Bar / Baz!", "foo_bar_baz"},
		{"leading trailing junk", "  --Alpha--  ", "alpha"},
		{"empty", "   ", "festival"},
		{"already slug", "alpha_beta", "alpha_beta"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, slugifyRef(tc.in))
		})
	}
}

func TestUniqueRef(t *testing.T) {
	taken := map[string]bool{"alpha": true, "alpha_2": true}
	assert.Equal(t, "beta", uniqueRef("beta", taken))
	assert.Equal(t, "alpha_3", uniqueRef("alpha", taken))
}

func TestBuildNodeDedupesRef(t *testing.T) {
	c := &chainpkg.Chain{
		Festivals: []chainpkg.FestivalNode{
			{Ref: "alpha", ID: "AL0001", Name: "alpha"},
		},
	}
	node := buildNode(c, "AL0002", "Alpha", []string{"projects/x"})
	assert.Equal(t, "alpha_2", node.Ref)
	assert.Equal(t, "AL0002", node.ID)
	assert.Equal(t, "Alpha", node.Name)
	assert.Equal(t, []string{"projects/x"}, node.Projects)
}

func testChain() *chainpkg.Chain {
	return &chainpkg.Chain{
		Metadata: chainpkg.Metadata{ID: "CH0001", Name: "demo"},
		Festivals: []chainpkg.FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
		Edges: []chainpkg.Edge{
			{From: "a", To: "b", Type: chainpkg.EdgeHard},
		},
	}
}

func TestResolvePrereqRef(t *testing.T) {
	c := testChain()

	gotRef, err := resolvePrereqRef(c, "a")
	require.NoError(t, err)
	assert.Equal(t, "a", gotRef)

	gotID, err := resolvePrereqRef(c, "B0001")
	require.NoError(t, err)
	assert.Equal(t, "b", gotID)

	_, err = resolvePrereqRef(c, "missing")
	assert.Error(t, err)
}

func TestBuildEdgeRejectsSelfEdge(t *testing.T) {
	c := testChain()
	_, err := buildEdge(c, "a", "a", chainpkg.EdgeHard, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "self-edge")
}

func TestBuildEdgeRejectsDuplicate(t *testing.T) {
	c := testChain()
	_, err := buildEdge(c, "a", "b", chainpkg.EdgeHard, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestBuildEdgeOK(t *testing.T) {
	c := testChain()
	e, err := buildEdge(c, "b", "a", chainpkg.EdgeSoft, "note here")
	require.NoError(t, err)
	assert.Equal(t, "b", e.From)
	assert.Equal(t, "a", e.To)
	assert.Equal(t, chainpkg.EdgeSoft, e.Type)
	assert.Equal(t, "note here", e.Note)
}

func TestParseEdgeType(t *testing.T) {
	hard, err := parseEdgeType("hard")
	require.NoError(t, err)
	assert.Equal(t, chainpkg.EdgeHard, hard)

	soft, err := parseEdgeType("soft")
	require.NoError(t, err)
	assert.Equal(t, chainpkg.EdgeSoft, soft)

	_, err = parseEdgeType("medium")
	assert.Error(t, err)
}

func TestPlaceInTrailingWaveNoOpWhenNoWaves(t *testing.T) {
	c := testChain()
	placeInTrailingWave(c, "a")
	assert.Empty(t, c.Waves)
}

func TestPlaceInTrailingWaveAppendsSequentialWave(t *testing.T) {
	c := testChain()
	c.Festivals = append(c.Festivals, chainpkg.FestivalNode{Ref: "g", ID: "G0001", Name: "gamma"})
	c.Edges = append(c.Edges, chainpkg.Edge{From: "a", To: "g", Type: chainpkg.EdgeHard})
	c.Waves = []chainpkg.Wave{
		{ID: 1, Name: "Foundation", Unlock: "none", Festivals: []string{"a"}},
		{ID: 2, Name: "Wave 2", Unlock: "a:completed", Festivals: []string{"b"}},
	}

	placeInTrailingWave(c, "g")

	require.Len(t, c.Waves, 3)
	last := c.Waves[2]
	assert.Equal(t, 3, last.ID)
	assert.Equal(t, []string{"g"}, last.Festivals)
	assert.Equal(t, "a:completed", last.Unlock)
}

func TestPlaceInTrailingWaveUnlockNoneForRoot(t *testing.T) {
	c := testChain()
	c.Festivals = append(c.Festivals, chainpkg.FestivalNode{Ref: "g", ID: "G0001", Name: "gamma"})
	c.Waves = []chainpkg.Wave{
		{ID: 1, Name: "Foundation", Unlock: "none", Festivals: []string{"a", "b"}},
	}

	placeInTrailingWave(c, "g")

	require.Len(t, c.Waves, 2)
	assert.Equal(t, "none", c.Waves[1].Unlock)
}

func TestWaveUnlockMultipleHardPrereqs(t *testing.T) {
	c := &chainpkg.Chain{
		Festivals: []chainpkg.FestivalNode{
			{Ref: "a"}, {Ref: "b"}, {Ref: "g"},
		},
		Edges: []chainpkg.Edge{
			{From: "a", To: "g", Type: chainpkg.EdgeHard},
			{From: "b", To: "g", Type: chainpkg.EdgeHard},
			{From: "a", To: "g", Type: chainpkg.EdgeSoft},
		},
	}
	assert.Equal(t, "a:completed AND b:completed", waveUnlock(c, "g"))
}
