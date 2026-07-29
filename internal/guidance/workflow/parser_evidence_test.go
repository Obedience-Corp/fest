package workflow

import (
	"strings"
	"testing"
)

// TestEvidenceBulletsAreNeverSilentlyDropped pins the defect this fixes: an
// Evidence bullet the parser cannot read as a path used to vanish, leaving a
// partial list that looked complete.
func TestEvidenceBulletsAreNeverSilentlyDropped(t *testing.T) {
	tests := []struct {
		name         string
		section      string
		wantPaths    []string
		wantUnparsed []string
	}{
		{
			name: "the scaffold placeholder is reported, not dropped",
			section: `## Step 1: PHASE GOAL

**Evidence:**
- PHASE_GOAL.md
- (attach each sequence's SEQUENCE_GOAL.md and task outputs relevant to this gate step)

**Checkpoint:** APPROVAL REQUIRED
`,
			wantPaths:    []string{"PHASE_GOAL.md"},
			wantUnparsed: []string{"(attach each sequence's SEQUENCE_GOAL.md and task outputs relevant to this gate step)"},
		},
		{
			name: "a path containing a space is reported",
			section: `## Step 1: GOAL

**Evidence:**
- outputs/my spec.md

**Checkpoint:** APPROVAL REQUIRED
`,
			wantUnparsed: []string{"outputs/my spec.md"},
		},
		{
			name: "a fully filled list reports nothing unparsed",
			section: `## Step 1: GOAL

**Evidence:**
- PHASE_GOAL.md
- 01_seq/outputs/spec.md
- 01_seq/results/testing-results.md

**Checkpoint:** APPROVAL REQUIRED
`,
			wantPaths: []string{"PHASE_GOAL.md", "01_seq/outputs/spec.md", "01_seq/results/testing-results.md"},
		},
	}

	p := &Parser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPaths, gotUnparsed := p.parseEvidence(tt.section)

			if strings.Join(gotPaths, ",") != strings.Join(tt.wantPaths, ",") {
				t.Errorf("paths = %v, want %v", gotPaths, tt.wantPaths)
			}
			if strings.Join(gotUnparsed, ",") != strings.Join(tt.wantUnparsed, ",") {
				t.Errorf("unparsed = %v, want %v", gotUnparsed, tt.wantUnparsed)
			}
		})
	}
}
