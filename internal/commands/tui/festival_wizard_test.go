package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Obedience-Corp/fest/internal/types"
)

func TestFestivalTypeSupportsSeedMatchesPipeline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ft   types.FestivalType
		want bool
	}{
		{
			name: "standard auto ingest",
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
			name: "non-auto ingest is not seedable",
			ft: types.FestivalType{
				Name: "custom",
				Phases: []types.PhaseSpec{
					{Name: "INGEST", Type: "ingest", Auto: false},
					{Name: "PLAN", Type: "planning", Auto: true},
				},
			},
			want: false,
		},
		{
			name: "name INGEST but type planning is not seedable",
			ft: types.FestivalType{
				Name: "weird",
				Phases: []types.PhaseSpec{
					{Name: "INGEST", Type: "planning", Auto: true},
				},
			},
			want: false,
		},
		{
			name: "ritual empty",
			ft:   types.FestivalType{Name: "ritual", Phases: nil},
			want: false,
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
	})
	if label != "standard*" {
		t.Fatalf("option label = %q, want standard*", label)
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

func TestNextStepAfterProject(t *testing.T) {
	t.Parallel()
	cfg := &types.FestivalTypesConfig{
		Types: []types.FestivalType{
			{
				Name: "standard",
				Phases: []types.PhaseSpec{
					{Name: "INGEST", Type: "ingest", Auto: true},
				},
			},
			{
				Name:          "implementation",
				SkipIngestion: true,
				Phases: []types.PhaseSpec{
					{Name: "IMPLEMENT", Type: "implementation", Auto: true},
				},
			},
		},
	}
	d := &festivalDraft{TypeName: "standard", Seed: "keep-me"}
	if step := nextStepAfterProject(cfg, d); step != 4 {
		t.Fatalf("standard next step = %d, want 4", step)
	}
	if d.Seed != "keep-me" {
		t.Fatalf("seed cleared for seedable type: %q", d.Seed)
	}
	d = &festivalDraft{TypeName: "implementation", Seed: "clear-me"}
	if step := nextStepAfterProject(cfg, d); step != 5 {
		t.Fatalf("implementation next step = %d, want 5", step)
	}
	if d.Seed != "" {
		t.Fatalf("seed not cleared for non-seedable type: %q", d.Seed)
	}
}

func TestBuildFestivalConfirmSummary(t *testing.T) {
	t.Parallel()
	cfg := &types.FestivalTypesConfig{
		Types: []types.FestivalType{{
			Name: "standard",
			Phases: []types.PhaseSpec{
				{Name: "INGEST", Type: "ingest", Auto: true},
				{Name: "PLAN", Type: "planning", Auto: true},
			},
		}},
	}
	d := &festivalDraft{
		TypeName: "standard",
		Name:     "ecommerce-mvp",
		Goal:     "Ship it",
		Project:  "projects/demo-app",
		Seed:     "brief",
		Tags:     "a,b",
	}
	sum := buildFestivalConfirmSummary(cfg, d)
	for _, part := range []string{"ecommerce-mvp", "standard", "planning/", "001-INGEST", "projects/demo-app", "yes (", "Ship it"} {
		if !strings.Contains(sum, part) {
			t.Fatalf("summary missing %q:\n%s", part, sum)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("short = %q", got)
	}
	// multi-byte runes must not be split mid-codepoint
	s := "日本語テスト文字列です"
	got := truncateRunes(s, 5)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid utf8 truncate: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis: %q", got)
	}
}

func TestTrimTagList(t *testing.T) {
	t.Parallel()
	got := trimTagList(" backend , commerce , ")
	if len(got) != 2 || got[0] != "backend" || got[1] != "commerce" {
		t.Fatalf("trimTagList = %#v", got)
	}
}

func TestResolveProjectMode(t *testing.T) {
	t.Parallel()
	projects := []string{"projects/a", "projects/b"}
	if got := resolveProjectMode(&festivalDraft{}, projects); got != projectModeSkip {
		t.Fatalf("empty project mode = %q", got)
	}
	if got := resolveProjectMode(&festivalDraft{Project: "projects/a"}, projects); got != projectModePick {
		t.Fatalf("listed path mode = %q", got)
	}
	if got := resolveProjectMode(&festivalDraft{Project: "custom/path"}, projects); got != projectModePath {
		t.Fatalf("custom path mode = %q", got)
	}
	if got := resolveProjectMode(&festivalDraft{Project: "x", ProjectMode: projectModePath}, projects); got != projectModePath {
		t.Fatalf("explicit mode not preserved: %q", got)
	}
}
