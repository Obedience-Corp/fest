//go:build !no_charm

package tui

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/types"
)

func TestFestivalTypeSupportsSeed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ft   types.FestivalType
		want bool
	}{
		{
			name: "standard with ingest",
			ft: types.FestivalType{
				Name: "standard",
				Phases: []types.PhaseSpec{
					{Name: "INGEST", Type: "ingest", Auto: true},
					{Name: "PLAN", Type: "planning", Auto: true},
				},
			},
			want: true,
		},
		{
			name: "implementation skip_ingestion",
			ft: types.FestivalType{
				Name:          "implementation",
				SkipIngestion: true,
				Phases: []types.PhaseSpec{
					{Name: "IMPLEMENT", Type: "implementation", Auto: true},
				},
			},
			want: false,
		},
		{
			name: "ritual empty",
			ft:   types.FestivalType{Name: "ritual", Phases: nil},
			want: false,
		},
		{
			name: "research with ingest",
			ft: types.FestivalType{
				Name: "research",
				Phases: []types.PhaseSpec{
					{Name: "INGEST", Type: "ingest", Auto: true},
				},
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := festivalTypeSupportsSeed(&tc.ft)
			if got != tc.want {
				t.Fatalf("festivalTypeSupportsSeed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFestivalTypeDest(t *testing.T) {
	t.Parallel()
	if got := festivalTypeDest(&types.FestivalType{Name: "ritual"}); got != "ritual" {
		t.Fatalf("ritual dest = %q, want ritual", got)
	}
	if got := festivalTypeDest(&types.FestivalType{Name: "standard"}); got != "planning" {
		t.Fatalf("standard dest = %q, want planning", got)
	}
}

func TestFestivalTypeOptionLabelIsShort(t *testing.T) {
	t.Parallel()
	label := festivalTypeOptionLabel(types.FestivalType{
		Name:        "standard",
		Description: "Default type with a very long description that must not appear in the select key",
		Default:     true,
		Phases: []types.PhaseSpec{
			{Name: "INGEST", Type: "ingest", Auto: true},
			{Name: "PLAN", Type: "planning", Auto: true},
		},
	})
	if label != "standard*" {
		t.Fatalf("option label = %q, want standard*", label)
	}
	if strings.Contains(label, "INGEST") || strings.Contains(label, "Default") {
		t.Fatalf("option label should not embed detail: %q", label)
	}
}

func TestTypesHelpNoteIncludesAutoPhases(t *testing.T) {
	t.Parallel()
	cfg := &types.FestivalTypesConfig{
		Types: []types.FestivalType{{
			Name:        "standard",
			Description: "Default type",
			Default:     true,
			Phases: []types.PhaseSpec{
				{Name: "INGEST", Type: "ingest", Auto: true},
				{Name: "PLAN", Type: "planning", Auto: true},
			},
		}},
	}
	note := typesHelpNote(cfg)
	for _, part := range []string{"standard*", "INGEST→PLAN", "planning/", "Default type"} {
		if !strings.Contains(note, part) {
			t.Fatalf("help note missing %q: %q", part, note)
		}
	}
}
