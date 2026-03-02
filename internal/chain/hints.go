package chain

import "context"

// HintEvaluation holds the result of evaluating sequence hints for a festival.
type HintEvaluation struct {
	// Festival is the chain ref being evaluated.
	Festival string

	// EarlyStart lists sequences that can begin before upstream deps are complete.
	EarlyStart []string

	// NeedsUpstream lists sequences that must wait for upstream completion.
	NeedsUpstream []string

	// ParallelGroups lists sequences that can run in parallel within the festival.
	ParallelGroups []string

	// Sequential lists sequences that must run after the parallel group.
	Sequential []string
}

// EvaluateHints determines which sequences within a festival can start early,
// which must wait for upstream completion, and how they can be parallelized.
// If the festival has no sequence hints defined, returns nil.
func EvaluateHints(ctx context.Context, c *Chain, festivalRef string, statuses map[string]FestivalStatus) *HintEvaluation {
	if err := ctx.Err(); err != nil {
		return nil
	}

	// Find the hint for this festival.
	var hint *SequenceHint
	for i := range c.SequenceHints {
		if c.SequenceHints[i].Festival == festivalRef {
			hint = &c.SequenceHints[i]
			break
		}
	}

	if hint == nil {
		return nil
	}

	eval := &HintEvaluation{
		Festival: festivalRef,
	}

	// Check if upstream deps are complete.
	upstreamComplete := areHardUpstreamComplete(c, festivalRef, statuses)

	if hint.EarlyStart != nil {
		if upstreamComplete {
			// All upstream done — all sequences can start freely.
			eval.EarlyStart = hint.EarlyStart.Sequences
		} else {
			// Upstream incomplete — only early_start sequences can begin.
			eval.EarlyStart = hint.EarlyStart.Sequences
		}
	}

	if hint.NeedsUpstream != nil {
		if upstreamComplete {
			// Upstream complete — these can now start too.
			eval.EarlyStart = append(eval.EarlyStart, hint.NeedsUpstream.Sequences...)
		} else {
			eval.NeedsUpstream = hint.NeedsUpstream.Sequences
		}
	}

	if hint.InternalParallelism != nil {
		eval.ParallelGroups = hint.InternalParallelism.Parallel
		eval.Sequential = hint.InternalParallelism.Then
	}

	return eval
}

// areHardUpstreamComplete checks if all hard upstream dependencies for a ref are completed.
func areHardUpstreamComplete(c *Chain, ref string, statuses map[string]FestivalStatus) bool {
	for _, e := range c.Edges {
		if e.To == ref && e.Type == EdgeHard {
			if statuses[e.From] != FestivalCompleted {
				return false
			}
		}
	}
	return true
}
