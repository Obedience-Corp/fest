package commit

import (
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
)

// The refusal message is fest's one chance to explain a blocked commit, so
// these tests pin the contract: every refusal names what was found, that
// nothing was staged, and working ways out (including camp commit
// --commit-large until fest grows its own flag). All assertions run against
// typed data rendered by fest; none depend on camp's error strings.

func TestGuardRefusalMessageBulkNamesEveryWayOut(t *testing.T) {
	blocked := &commitkit.GuardBlockedError{
		Kind: commitkit.Bulk,
		Violations: []commitkit.GuardViolation{
			{Kind: commitkit.Bulk, CommonPrefix: "node_modules", Count: 1204, TotalBytes: 220 << 20},
		},
	}

	msg := guardRefusalMessage(blocked)

	for _, want := range []string{
		"1204 untracked files",
		"node_modules/",
		"nothing was staged",
		"echo 'node_modules/' >> .gitignore",
		"camp artifacts add node_modules",
		"camp commit --commit-large",
		"camp settings set local.commit.guards.bulk off",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("bulk refusal missing %q in:\n%s", want, msg)
		}
	}
}

func TestGuardRefusalMessageLargeFilesNamesEveryWayOut(t *testing.T) {
	blocked := &commitkit.GuardBlockedError{
		Kind: commitkit.OverThreshold,
		Violations: []commitkit.GuardViolation{
			{Kind: commitkit.OverThreshold, Path: "media/clip.bin", Size: 900 << 20},
		},
		Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
	}

	msg := guardRefusalMessage(blocked)

	for _, want := range []string{
		"1 file over the 100.0 MB limit",
		"media/clip.bin",
		"nothing was staged",
		"camp artifacts add media",
		"camp commit --commit-large",
		"camp settings set local.commit.guards.large_files auto",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("large-file refusal missing %q in:\n%s", want, msg)
		}
	}
}

func TestReportStageOutcomeSaysWhatTheGuardDid(t *testing.T) {
	var b strings.Builder
	reportStageOutcome(&b, &commitkit.StageOutcome{
		Excluded: []commitkit.GuardViolation{
			{Kind: commitkit.OverThreshold, Path: "render.bin", Size: 512 << 20},
		},
		Reported: []commitkit.GuardViolation{
			{Kind: commitkit.TrackedGrowth, Path: "fixtures/golden.json", Size: 150 << 20},
		},
		Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
	})

	out := b.String()
	for _, want := range []string{
		"Kept out of git: render.bin",
		"camp commit --commit-large",
		"Tracked file grew past 100.0 MB: fixtures/golden.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("outcome report missing %q in:\n%s", want, out)
		}
	}
}

func TestReportStageOutcomeIsSilentWhenThereIsNothingToSay(t *testing.T) {
	var b strings.Builder
	reportStageOutcome(&b, nil)
	reportStageOutcome(&b, &commitkit.StageOutcome{})
	if b.Len() != 0 {
		t.Errorf("an uneventful stage must print nothing, got:\n%s", b.String())
	}
}
