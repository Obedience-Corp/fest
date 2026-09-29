package progress

import "testing"

func TestSplitBlockersUsesTheFlagNotSubtraction(t *testing.T) {
	open, deferred := SplitBlockers([]*TaskProgress{
		{TaskID: "a", BlockerDeferred: false},
		nil,
		{TaskID: "b", BlockerDeferred: true},
		{TaskID: "c", BlockerDeferred: false},
	})

	if len(open) != 2 || open[0].TaskID != "a" || open[1].TaskID != "c" {
		t.Errorf("open = %+v, want a and c in the order they were given", open)
	}
	if len(deferred) != 1 || deferred[0].TaskID != "b" {
		t.Errorf("deferred = %+v, want b", deferred)
	}
}

func TestSplitBlockersReturnsNothingForAnEmptySet(t *testing.T) {
	open, deferred := SplitBlockers(nil)
	if open != nil || deferred != nil {
		t.Errorf("SplitBlockers(nil) = %v, %v, want nil and nil", open, deferred)
	}
}
