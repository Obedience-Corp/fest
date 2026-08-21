package commit

import (
	"testing"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
)

func TestCampaignTagComponents(t *testing.T) {
	t.Parallel()

	const (
		campaignName = "obey-campaign"
		campaignID   = "8deed8b4"
	)

	tests := []struct {
		name    string
		festRef string
		pos     position
		want    string
	}{
		{
			name: "no festival reference",
			want: "[obey-campaign:8deed8b4]",
		},
		{
			name:    "festival reference without a position",
			festRef: "FE-CC0008",
			want:    "[obey-campaign:8deed8b4-FE-CC0008]",
		},
		{
			name:    "sequence without a phase is never emitted",
			festRef: "FE-CC0008",
			pos:     position{Sequence: "02"},
			want:    "[obey-campaign:8deed8b4-FE-CC0008]",
		},
		{
			name:    "phase only",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001"},
			want:    "[obey-campaign:8deed8b4-FE-CC0008-PH-001]",
		},
		{
			name:    "phase and sequence",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001", Sequence: "02"},
			want:    "[obey-campaign:8deed8b4-FE-CC0008-PH-001-SQ-02]",
		},
		{
			name: "position without a festival reference is dropped",
			pos:  position{Phase: "001", Sequence: "02"},
			want: "[obey-campaign:8deed8b4]",
		},
		{
			name:    "task reference carries a position too",
			festRef: "FE-FEST-123456",
			pos:     position{Phase: "001", Sequence: "02"},
			want:    "[obey-campaign:8deed8b4-FE-FEST-123456-PH-001-SQ-02]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := commitkit.FormatTag(campaignTagComponents(campaignName, campaignID, tt.festRef, tt.pos))
			if got != tt.want {
				t.Errorf("campaign tag = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCampaignTagComponents_UnpositionedMatchesLegacyEmitter pins the no-position
// tag to the output the positional emitter produced before PH-/SQ- existed.
func TestCampaignTagComponents_UnpositionedMatchesLegacyEmitter(t *testing.T) {
	t.Parallel()

	const (
		campaignName = "obey-campaign"
		campaignID   = "8deed8b4"
	)

	tests := []struct {
		name    string
		festRef string
		festID  string
	}{
		{name: "no festival reference"},
		{name: "festival reference", festRef: "FE-CC0008", festID: "CC0008"},
		{name: "task reference", festRef: "FE-FEST-123456", festID: "FEST-123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			legacy := commitkit.FormatContextTagsFullNamed(campaignName, campaignID, "", tt.festID, "")
			got := commitkit.FormatTag(campaignTagComponents(campaignName, campaignID, tt.festRef, position{}))
			if got != legacy {
				t.Errorf("campaign tag = %q, want legacy %q", got, legacy)
			}
		})
	}
}

func TestFestRefWithPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		festRef string
		pos     position
		want    string
	}{
		{name: "no reference", pos: position{Phase: "001", Sequence: "02"}},
		{name: "no position", festRef: "FE-CC0008", want: "FE-CC0008"},
		{
			name:    "sequence without a phase is never emitted",
			festRef: "FE-CC0008",
			pos:     position{Sequence: "02"},
			want:    "FE-CC0008",
		},
		{
			name:    "phase only",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001"},
			want:    "FE-CC0008-PH-001",
		},
		{
			name:    "phase and sequence",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001", Sequence: "02"},
			want:    "FE-CC0008-PH-001-SQ-02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := festRefWithPosition(tt.festRef, tt.pos); got != tt.want {
				t.Errorf("festRefWithPosition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFestCommitMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		festRef string
		pos     position
		want    string
	}{
		{name: "untagged", want: "feat: update camp scaffold"},
		{
			name:    "reference without a position",
			festRef: "FE-CC0008",
			want:    "[FE-CC0008] feat: update camp scaffold",
		},
		{
			name:    "reference with a position",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001", Sequence: "02"},
			want:    "[FE-CC0008-PH-001-SQ-02] feat: update camp scaffold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := festCommitMessage(tt.festRef, tt.pos, "feat: update camp scaffold"); got != tt.want {
				t.Errorf("festCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFestivalRootCommitMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		campaignTag string
		festRef     string
		pos         position
		want        string
	}{
		{name: "no tag and no reference", want: "fest: update camp scaffold"},
		{
			name:    "reference without a position",
			festRef: "FE-CC0008",
			want:    "[FE-CC0008] fest: update camp scaffold",
		},
		{
			name:    "reference with a position",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001", Sequence: "02"},
			want:    "[FE-CC0008-PH-001-SQ-02] fest: update camp scaffold",
		},
		{
			name:    "reference with a phase only",
			festRef: "FE-CC0008",
			pos:     position{Phase: "001"},
			want:    "[FE-CC0008-PH-001] fest: update camp scaffold",
		},
		{
			name:        "campaign tag already carries the position",
			campaignTag: "[obey-campaign:8deed8b4-FE-CC0008-PH-001-SQ-02]",
			festRef:     "FE-CC0008",
			pos:         position{Phase: "001", Sequence: "02"},
			want:        "[obey-campaign:8deed8b4-FE-CC0008-PH-001-SQ-02] fest: update camp scaffold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := festivalRootCommitMessage(tt.campaignTag, tt.festRef, tt.pos, "update camp scaffold")
			if got != tt.want {
				t.Errorf("festivalRootCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
