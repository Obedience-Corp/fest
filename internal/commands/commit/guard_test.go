package commit

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
)

// errGuardUnavailable stands in for whatever camp reports when the guard cannot
// read its thresholds; fest only has to pass it through.
var errGuardUnavailable = stderrors.New("no settings file")

// The refusal message is fest's one chance to explain a blocked commit, so
// these tests pin the contract: every refusal names what was found, that
// nothing was staged, and working ways out. The commit-it-anyway retry is
// fest's own flag on every branch, because every stage fest performs carries
// --commit-large into the guard decision. All assertions run against typed
// data rendered by fest; none depend on camp's error strings.

func TestGuardRefusalMessageNamesEveryWayOut(t *testing.T) {
	tests := []struct {
		name    string
		blocked *commitkit.GuardBlockedError
		want    []string
		notWant []string
	}{
		{
			name: "bulk",
			blocked: &commitkit.GuardBlockedError{
				Kind: commitkit.Bulk,
				Violations: []commitkit.GuardViolation{
					{Kind: commitkit.Bulk, CommonPrefix: "node_modules", Count: 1204, TotalBytes: 220 << 20},
				},
			},
			want: []string{
				"1204 untracked files",
				"node_modules/",
				"nothing was staged",
				"echo 'node_modules/' >> .gitignore",
				"camp artifacts add node_modules",
				"fest commit --commit-large",
				"camp settings set local.commit.guards.bulk off",
			},
			notWant: []string{"camp commit --commit-large"},
		},
		{
			name: "over threshold",
			blocked: &commitkit.GuardBlockedError{
				Kind: commitkit.OverThreshold,
				Violations: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "media/clip.bin", Size: 900 << 20},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"1 file over the 100.0 MB limit",
				"media/clip.bin",
				"nothing was staged",
				"camp artifacts add media",
				"fest commit --commit-large",
				"camp settings set local.commit.guards.large_files auto",
			},
			notWant: []string{"camp commit --commit-large"},
		},
		{
			name: "over threshold with several files",
			blocked: &commitkit.GuardBlockedError{
				Kind: commitkit.OverThreshold,
				Violations: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "media/clip.bin", Size: 900 << 20},
					{Kind: commitkit.OverThreshold, Path: "data/dump.sql", Size: 300 << 20},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"2 files over the 100.0 MB limit",
				"media/clip.bin",
				"data/dump.sql",
				"camp artifacts add media",
				"camp artifacts add data",
				"fest commit --commit-large",
			},
			notWant: []string{"camp commit --commit-large"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := guardRefusalMessage(tt.blocked)

			for _, want := range tt.want {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal missing %q in:\n%s", want, msg)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(msg, notWant) {
					t.Errorf("refusal must not send the user to another binary; found %q in:\n%s", notWant, msg)
				}
			}
		})
	}
}

func TestReportStageOutcome(t *testing.T) {
	tests := []struct {
		name    string
		outcome *commitkit.StageOutcome
		want    []string
		silent  bool
	}{
		{
			name:    "nil outcome says nothing",
			outcome: nil,
			silent:  true,
		},
		{
			name:    "uneventful stage says nothing",
			outcome: &commitkit.StageOutcome{},
			silent:  true,
		},
		{
			name: "exclusion carries fest's own retry",
			outcome: &commitkit.StageOutcome{
				Excluded: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "render.bin", Size: 512 << 20},
				},
				Reported: []commitkit.GuardViolation{
					{Kind: commitkit.TrackedGrowth, Path: "fixtures/golden.json", Size: 150 << 20},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"Kept out of git: render.bin",
				"fest commit --commit-large",
				"Tracked file grew past 100.0 MB: fixtures/golden.json",
			},
		},
		{
			name: "a guard that could not run is said out loud",
			outcome: &commitkit.StageOutcome{
				Unavailable: errGuardUnavailable,
			},
			want: []string{"Staged without the size and bulk guard", "no settings file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			reportStageOutcome(&b, tt.outcome)

			out := b.String()
			if tt.silent {
				if out != "" {
					t.Errorf("expected no output, got:\n%s", out)
				}
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("outcome report missing %q in:\n%s", want, out)
				}
			}
		})
	}
}
