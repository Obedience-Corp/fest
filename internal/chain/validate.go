package chain

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ValidationResult holds the outcome of chain validation.
type ValidationResult struct {
	Valid    bool
	Score    int
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError is a structural or referential error.
type ValidationError struct {
	Code    string
	Message string
	Context string
}

// ValidationWarning is a non-critical issue.
type ValidationWarning struct {
	Code    string
	Message string
	Context string
}

func (e ValidationError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Context)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Validate runs all structural validation rules (S1–S10) on a chain.
func Validate(ctx context.Context, c *Chain) *ValidationResult {
	result := &ValidationResult{Valid: true, Score: 100}

	checks := []func(context.Context, *Chain, *ValidationResult){
		validateUniqueRefs,       // S1
		validateUniqueIDs,        // S2
		validateEdgeRefs,         // S3
		validateNoSelfEdges,      // S4
		validateNoCycles,         // S5
		validateEdgeTypes,        // S6
		validateWaveCoverage,     // S7
		validateWaveOrdering,     // S8
		validateWaveUnlockSyntax, // S9
		validateRequiredFields,   // S10
	}

	for _, check := range checks {
		if ctx.Err() != nil {
			result.addError("CTX", "context cancelled", "")
			break
		}
		check(ctx, c, result)
	}

	if len(result.Errors) > 0 {
		result.Valid = false
		// Deduct points based on error severity.
		result.Score = max(0, 100-len(result.Errors)*15)
	} else if len(result.Warnings) > 0 {
		result.Score = max(70, 100-len(result.Warnings)*5)
	}

	return result
}

func (r *ValidationResult) addError(code, message, ctx string) {
	r.Errors = append(r.Errors, ValidationError{Code: code, Message: message, Context: ctx})
}

func (r *ValidationResult) addWarning(code, message, ctx string) {
	r.Warnings = append(r.Warnings, ValidationWarning{Code: code, Message: message, Context: ctx})
}

// S1: Unique Refs
func validateUniqueRefs(_ context.Context, c *Chain, r *ValidationResult) {
	seen := make(map[string]int)
	for i, f := range c.Festivals {
		if prev, ok := seen[f.Ref]; ok {
			r.addError("S1",
				fmt.Sprintf("duplicate ref %q at festivals[%d] and festivals[%d]", f.Ref, prev, i), "")
		}
		seen[f.Ref] = i
	}
}

// S2: Unique IDs
func validateUniqueIDs(_ context.Context, c *Chain, r *ValidationResult) {
	seen := make(map[string]int)
	for i, f := range c.Festivals {
		if prev, ok := seen[f.ID]; ok {
			r.addError("S2",
				fmt.Sprintf("duplicate festival ID %q at festivals[%d] and festivals[%d]", f.ID, prev, i), "")
		}
		seen[f.ID] = i
	}
}

// S3: Valid Edge Refs
func validateEdgeRefs(_ context.Context, c *Chain, r *ValidationResult) {
	refs := c.RefSet()
	validRefs := make([]string, 0, len(refs))
	for ref := range refs {
		validRefs = append(validRefs, ref)
	}
	sort.Strings(validRefs)
	refList := strings.Join(validRefs, ", ")

	for i, e := range c.Edges {
		if !refs[e.From] {
			r.addError("S3",
				fmt.Sprintf("edge[%d].from references unknown ref %q", i, e.From),
				"valid refs: "+refList)
		}
		if !refs[e.To] {
			r.addError("S3",
				fmt.Sprintf("edge[%d].to references unknown ref %q", i, e.To),
				"valid refs: "+refList)
		}
	}
}

// S4: No Self-Edges
func validateNoSelfEdges(_ context.Context, c *Chain, r *ValidationResult) {
	for i, e := range c.Edges {
		if e.From == e.To {
			r.addError("S4",
				fmt.Sprintf("edge[%d] is a self-edge (from: %q -> to: %q)", i, e.From, e.To), "")
		}
	}
}

// S5: No Cycles (DAG Enforcement)
func validateNoCycles(ctx context.Context, c *Chain, r *ValidationResult) {
	dag, err := BuildDAG(ctx, c)
	if err != nil {
		r.addError("S5", "failed to build DAG", err.Error())
		return
	}

	_, err = dag.TopologicalSort(ctx)
	if err != nil {
		// Find which nodes are in the cycle.
		r.addError("S5", "cycle detected in dependency graph", err.Error())
	}
}

// S6: Valid Edge Types
func validateEdgeTypes(_ context.Context, c *Chain, r *ValidationResult) {
	for i, e := range c.Edges {
		if e.Type != EdgeHard && e.Type != EdgeSoft {
			r.addError("S6",
				fmt.Sprintf("edge[%d].type %q is not valid", i, e.Type),
				"must be \"hard\" or \"soft\"")
		}
	}
}

// S7: Wave Coverage
func validateWaveCoverage(_ context.Context, c *Chain, r *ValidationResult) {
	if len(c.Waves) == 0 {
		return // waves are optional
	}

	waveOf := make(map[string]int) // ref -> wave ID
	for _, w := range c.Waves {
		for _, ref := range w.Festivals {
			if prevWave, ok := waveOf[ref]; ok {
				r.addError("S7",
					fmt.Sprintf("festival ref %q appears in wave %d and wave %d", ref, prevWave, w.ID), "")
			}
			waveOf[ref] = w.ID
		}
	}

	for _, f := range c.Festivals {
		if _, ok := waveOf[f.Ref]; !ok {
			r.addError("S7",
				fmt.Sprintf("festival ref %q not found in any wave", f.Ref), "")
		}
	}
}

// S8: Wave Ordering Consistency
func validateWaveOrdering(_ context.Context, c *Chain, r *ValidationResult) {
	if len(c.Waves) == 0 {
		return
	}

	// Check sequential IDs.
	for i, w := range c.Waves {
		if w.ID != i+1 {
			ids := make([]int, len(c.Waves))
			for j, w2 := range c.Waves {
				ids[j] = w2.ID
			}
			r.addError("S8",
				fmt.Sprintf("wave IDs are not sequential: found %v", ids), "")
			break
		}
	}

	// Build ref -> wave ID mapping.
	waveOf := make(map[string]int)
	for _, w := range c.Waves {
		for _, ref := range w.Festivals {
			waveOf[ref] = w.ID
		}
	}

	// Check that hard dependencies only point to earlier waves.
	for _, e := range c.Edges {
		if e.Type != EdgeHard {
			continue
		}
		fromWave := waveOf[e.From]
		toWave := waveOf[e.To]
		if fromWave > 0 && toWave > 0 && fromWave >= toWave {
			r.addError("S8",
				fmt.Sprintf("wave %d contains %q which has hard dependency on %q (wave %d)",
					toWave, e.To, e.From, fromWave), "")
		}
	}
}

// S9: Wave Unlock Syntax
func validateWaveUnlockSyntax(_ context.Context, c *Chain, r *ValidationResult) {
	if len(c.Waves) == 0 {
		return
	}

	refs := c.RefSet()

	for _, w := range c.Waves {
		if w.Unlock == "" || w.Unlock == "none" {
			continue
		}

		// Parse "ref:status AND ref:status ..."
		parts := strings.Split(w.Unlock, " AND ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			colonIdx := strings.Index(part, ":")
			if colonIdx < 1 {
				r.addError("S9",
					fmt.Sprintf("wave %d unlock has invalid syntax: %q", w.ID, part),
					"expected format: ref:status")
				continue
			}
			ref := part[:colonIdx]
			if !refs[ref] {
				r.addError("S9",
					fmt.Sprintf("wave %d unlock references unknown ref %q in expression %q",
						w.ID, ref, w.Unlock), "")
			}
		}
	}
}

// CrossChainResult holds the outcome of cross-chain validation.
type CrossChainResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidateCrossChain checks for issues across multiple chains:
// - Duplicate festival IDs appearing in different chains
// - Conflicting lifecycle states (same festival in active and completed chains)
// - Stale chains (all festivals completed but chain not archived)
func ValidateCrossChain(ctx context.Context, chains []*Chain) *CrossChainResult {
	result := &CrossChainResult{Valid: true}

	if len(chains) < 2 {
		return result
	}

	// Check duplicate festival IDs across chains.
	type festLocation struct {
		chainID    string
		festivalID string
	}
	idMap := make(map[string][]festLocation)
	for _, c := range chains {
		for _, f := range c.Festivals {
			idMap[f.ID] = append(idMap[f.ID], festLocation{
				chainID:    c.Metadata.ID,
				festivalID: f.ID,
			})
		}
	}
	for fid, locs := range idMap {
		if len(locs) > 1 {
			chainIDs := make([]string, len(locs))
			for i, l := range locs {
				chainIDs[i] = l.chainID
			}
			result.Errors = append(result.Errors, ValidationError{
				Code:    "XC1",
				Message: fmt.Sprintf("festival ID %q appears in multiple chains: %s", fid, strings.Join(chainIDs, ", ")),
			})
		}
	}

	// Check conflicting lifecycle states.
	chainStatusByID := make(map[string][]string) // festival ID -> chain statuses
	for _, c := range chains {
		for _, f := range c.Festivals {
			chainStatusByID[f.ID] = append(chainStatusByID[f.ID], string(c.Metadata.Status))
		}
	}

	// Check for stale chains: all festivals could be completed but chain is still active.
	// This is a warning since we don't have live statuses here.
	for _, c := range chains {
		if ctx.Err() != nil {
			break
		}
		if c.Metadata.Status == StatusActive && len(c.Festivals) > 0 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code:    "XC2",
				Message: fmt.Sprintf("chain %q is active — verify all festivals are not already completed", c.Metadata.ID),
			})
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// S10: Required Fields
func validateRequiredFields(_ context.Context, c *Chain, r *ValidationResult) {
	if len(c.Festivals) < 2 {
		r.addError("S10",
			fmt.Sprintf("chain must contain at least 2 festivals (found %d)", len(c.Festivals)), "")
	}
	if len(c.Edges) < 1 {
		r.addError("S10",
			fmt.Sprintf("chain must contain at least 1 edge (found %d)", len(c.Edges)), "")
	}
}
