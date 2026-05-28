package standalone

import (
	"context"
	"fmt"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wfparser "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

// ValidateWorkflowDoc ensures WORKFLOW.md is parseable and has at least one
// step. Callers use this BEFORE creating any localstore state so a malformed
// or empty WORKFLOW.md cannot leave the user with a half-initialized
// .workflow/ directory (D030 finding #7).
func ValidateWorkflowDoc(ctx context.Context, path string) error {
	n, err := wfparser.NewParser().StepCount(ctx, path)
	if err != nil {
		return festerrors.Wrap(err, "parsing WORKFLOW.md").WithField("path", path)
	}
	if n == 0 {
		return festerrors.Validation("WORKFLOW.md has no parseable steps").
			WithField("path", path).
			WithHint("add at least one '## Step 1:' heading")
	}
	return nil
}

// ValidateStepNumbering checks that step headings in WORKFLOW.md form a
// contiguous 1..N sequence with no gaps, duplicates, or off-by-one starts
// (e.g. Step 0). It is intentionally narrow: it does NOT validate section
// names, state-tracking syntax, or checkpoints. WORKFLOW.md is the lightweight
// customizable surface; anything stricter would fight that design. State for
// tracking lives in .fest/progress_events.jsonl, not in the doc.
func ValidateStepNumbering(ctx context.Context, path string) error {
	steps, err := wfparser.NewParser().Parse(ctx, path)
	if err != nil {
		return festerrors.Wrap(err, "parsing WORKFLOW.md").WithField("path", path)
	}
	if len(steps) == 0 {
		return nil
	}

	nums := make([]int, len(steps))
	for i, s := range steps {
		nums[i] = s.Number
	}

	if msg, ok := checkContiguousFromOne(nums); !ok {
		return festerrors.Validation(msg).
			WithField("path", path).
			WithField("found", formatSequence(nums)).
			WithHint(remediationHint(nums))
	}
	return nil
}

func checkContiguousFromOne(nums []int) (string, bool) {
	if len(nums) == 0 {
		return "", true
	}
	if nums[0] != 1 {
		return "WORKFLOW.md: step numbering must start at 1", false
	}
	seen := make(map[int]bool, len(nums))
	for i, n := range nums {
		if seen[n] {
			return fmt.Sprintf("WORKFLOW.md: duplicate step number %d", n), false
		}
		seen[n] = true
		if i > 0 && n != nums[i-1]+1 {
			return fmt.Sprintf("WORKFLOW.md: step numbering is not contiguous (jumped from %d to %d)", nums[i-1], n), false
		}
	}
	return "", true
}

func formatSequence(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("Step %d", n)
	}
	return strings.Join(parts, ", ")
}

func remediationHint(nums []int) string {
	if len(nums) == 0 {
		return "add a '## Step 1:' heading"
	}
	target := fmt.Sprintf("Step 1..%d", len(nums))
	if nums[0] == 0 {
		return fmt.Sprintf("Fix one of: a) renumber to %s. b) merge Step 0's content into Step 1 and delete the heading.", target)
	}
	return fmt.Sprintf("renumber headings so they form %s with no gaps or duplicates.", target)
}
