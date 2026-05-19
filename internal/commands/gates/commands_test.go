package gates

import (
	"reflect"
	"testing"

	gatescore "github.com/Obedience-Corp/fest/internal/gates"
)

func TestOrderedPhaseBuckets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   map[string][]gatescore.GateTask
		want []string
	}{
		{
			name: "only implementation",
			in: map[string][]gatescore.GateTask{
				"implementation": {{ID: "testing"}},
			},
			want: []string{"implementation"},
		},
		{
			name: "custom bucket appears after canonical",
			in: map[string][]gatescore.GateTask{
				"implementation": {{ID: "testing"}},
				"problem-mining": {{ID: "artifact-check"}},
			},
			want: []string{"implementation", "problem-mining"},
		},
		{
			name: "multiple custom buckets sorted alphabetically",
			in: map[string][]gatescore.GateTask{
				"zeta":           {{ID: "z"}},
				"problem-mining": {{ID: "artifact-check"}},
				"alpha":          {{ID: "a"}},
			},
			want: []string{"alpha", "problem-mining", "zeta"},
		},
		{
			name: "canonical buckets render in declared order",
			in: map[string][]gatescore.GateTask{
				"review":         {{ID: "r"}},
				"implementation": {{ID: "i"}},
				"research":       {{ID: "rs"}},
			},
			want: []string{"implementation", "research", "review"},
		},
		{
			name: "other bucket is treated as custom",
			in: map[string][]gatescore.GateTask{
				"implementation": {{ID: "testing"}},
				"other":          {{ID: "custom"}},
			},
			want: []string{"implementation", "other"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := orderedPhaseBuckets(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("orderedPhaseBuckets() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractPhaseFromTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		template string
		want     string
	}{
		{"gates/implementation/QUALITY_GATE_TESTING", "implementation"},
		{"gates/problem-mining/QUALITY_GATE_ARTIFACT_CHECK", "problem-mining"},
		{"gates/problem-mining/QUALITY_GATE_ARTIFACT_CHECK.md", "problem-mining"},
		{"gates/research/QUALITY_GATE_FOO", "research"},
		{"agent/gates/implementation/testing", "implementation"},
		{"", "other"},
	}

	for _, tc := range tests {
		t.Run(tc.template, func(t *testing.T) {
			t.Parallel()
			if got := extractPhaseFromTemplate(tc.template); got != tc.want {
				t.Fatalf("extractPhaseFromTemplate(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}
