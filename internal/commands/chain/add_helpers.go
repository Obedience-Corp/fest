package chain

import (
	"fmt"
	"regexp"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyRef(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "festival"
	}
	return s
}

func uniqueRef(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if !taken[candidate] {
			return candidate
		}
	}
}

func buildNode(c *chainpkg.Chain, id, name string, projects []string) chainpkg.FestivalNode {
	ref := uniqueRef(slugifyRef(name), c.RefSet())
	return chainpkg.FestivalNode{
		Ref:      ref,
		ID:       id,
		Name:     name,
		Projects: projects,
	}
}

func resolvePrereqRef(c *chainpkg.Chain, after string) (string, error) {
	if node := c.FestivalByRef(after); node != nil {
		return after, nil
	}
	for _, f := range c.Festivals {
		if f.ID == after {
			return f.Ref, nil
		}
	}
	return "", errors.Validation("prerequisite festival not found in chain").
		WithField("after", after).
		WithField("chainID", c.Metadata.ID).
		WithHint("pass the ref or id of a festival already in the chain")
}

func buildEdge(c *chainpkg.Chain, fromRef, toRef string, edgeType chainpkg.EdgeType, note string) (chainpkg.Edge, error) {
	if fromRef == toRef {
		return chainpkg.Edge{}, errors.Validation("cannot add a self-edge").
			WithField("ref", fromRef)
	}
	for _, e := range c.Edges {
		if e.From == fromRef && e.To == toRef {
			return chainpkg.Edge{}, errors.Validation("duplicate edge").
				WithField("from", fromRef).
				WithField("to", toRef)
		}
	}
	return chainpkg.Edge{
		From: fromRef,
		To:   toRef,
		Type: edgeType,
		Note: note,
	}, nil
}

func parseEdgeType(s string) (chainpkg.EdgeType, error) {
	switch chainpkg.EdgeType(s) {
	case chainpkg.EdgeHard:
		return chainpkg.EdgeHard, nil
	case chainpkg.EdgeSoft:
		return chainpkg.EdgeSoft, nil
	default:
		return "", errors.Validation(fmt.Sprintf("invalid edge type %q", s)).
			WithHint("use --type hard or --type soft")
	}
}

// placeInTrailingWave appends the added ref to a new trailing wave when the chain
// already uses waves, so coverage (S7) and ordering (S8) still hold: the new wave
// gets the next sequential id, and its unlock requires every hard prerequisite of
// the added festival to be completed, which by construction sits in an earlier
// wave. Chains without waves are left untouched.
func placeInTrailingWave(c *chainpkg.Chain, addedRef string) {
	if len(c.Waves) == 0 {
		return
	}

	maxID := 0
	for _, w := range c.Waves {
		if w.ID > maxID {
			maxID = w.ID
		}
	}

	c.Waves = append(c.Waves, chainpkg.Wave{
		ID:        maxID + 1,
		Name:      fmt.Sprintf("Wave %d", maxID+1),
		Unlock:    waveUnlock(c, addedRef),
		Festivals: []string{addedRef},
	})
}

func waveUnlock(c *chainpkg.Chain, addedRef string) string {
	var conds []string
	seen := make(map[string]bool)
	for _, e := range c.Edges {
		if e.To != addedRef || e.Type != chainpkg.EdgeHard {
			continue
		}
		if seen[e.From] {
			continue
		}
		seen[e.From] = true
		conds = append(conds, e.From+":completed")
	}
	if len(conds) == 0 {
		return "none"
	}
	return strings.Join(conds, " AND ")
}
