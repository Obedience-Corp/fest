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
		{
			name: "nested repository",
			blocked: &commitkit.GuardBlockedError{
				Kind: commitkit.NestedRepo,
				Violations: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "vendored", Head: "b2d7b75"},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"1 nested repository",
				"vendored (a git repository at b2d7b75, not declared in .gitmodules)",
				"nothing was staged",
				"git submodule add <url> vendored",
				"fest commit --commit-nested",
				"camp settings set local.commit.guards.nested_repos off",
			},
			// A nested repository is not a size problem: the size limit and
			// its remedies must not appear, or the user is handed a way out
			// that cannot work.
			notWant: []string{
				"camp commit --commit-large",
				"over the 100.0 MB limit",
				"camp artifacts add",
			},
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
			name: "a nested repository is never dropped silently",
			outcome: &commitkit.StageOutcome{
				NestedRepos: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "vendored", Head: "b2d7b75"},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"Kept out of git: vendored (a git repository at b2d7b75, not declared in .gitmodules)",
				"fest commit --commit-nested",
				"git submodule add <url> vendored",
				"camp settings set local.commit.guards.nested_repos off",
			},
		},
		{
			name: "a nested repository with no commits yet still reports",
			outcome: &commitkit.StageOutcome{
				NestedRepos: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "scratch"},
				},
			},
			want: []string{
				"Kept out of git: scratch (a git repository with no commits yet, not declared in .gitmodules)",
				"fest commit --commit-nested",
			},
		},
		{
			name: "an excluded file and a nested repository each get their own line",
			outcome: &commitkit.StageOutcome{
				Excluded: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "render.bin", Size: 512 << 20},
				},
				NestedRepos: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "vendored", Head: "b2d7b75"},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"Kept out of git: render.bin",
				"fest commit --commit-large",
				"Kept out of git: vendored",
				"fest commit --commit-nested",
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

// A commit whose every change the guard held back must never surface as a bare
// "no changes to commit": that is the silent-exclusion failure the guard is
// there to prevent.
func TestAllExcludedMessage(t *testing.T) {
	tests := []struct {
		name    string
		outcome *commitkit.StageOutcome
		want    []string
		notWant []string
	}{
		{
			name: "nested repository only",
			outcome: &commitkit.StageOutcome{
				NestedRepos: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "vendored", Head: "b2d7b75"},
				},
			},
			want: []string{
				"nothing left to commit",
				"vendored (a git repository at b2d7b75, not declared in .gitmodules)",
				"fest commit --commit-nested",
			},
			notWant: []string{"fest commit --commit-large -m"},
		},
		{
			name: "over-threshold file only",
			outcome: &commitkit.StageOutcome{
				Excluded: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "render.bin", Size: 512 << 20},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"nothing left to commit",
				"render.bin (512.0 MB, over the 100.0 MB limit)",
				"fest commit --commit-large",
			},
			notWant: []string{"--commit-nested"},
		},
		{
			name: "both kinds need both flags",
			outcome: &commitkit.StageOutcome{
				Excluded: []commitkit.GuardViolation{
					{Kind: commitkit.OverThreshold, Path: "render.bin", Size: 512 << 20},
				},
				NestedRepos: []commitkit.GuardViolation{
					{Kind: commitkit.NestedRepo, Path: "vendored", Head: "b2d7b75"},
				},
				Limits: commitkit.GuardLimits{MaxFileSize: 100 << 20},
			},
			want: []string{
				"render.bin",
				"vendored",
				"fest commit --commit-large --commit-nested",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := allExcludedMessage(tt.outcome)

			for _, want := range tt.want {
				if !strings.Contains(msg, want) {
					t.Errorf("message missing %q in:\n%s", want, msg)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(msg, notWant) {
					t.Errorf("message must not offer an irrelevant remedy; found %q in:\n%s", notWant, msg)
				}
			}
		})
	}
}

func TestNewCommitCommand_CommitNestedFlag(t *testing.T) {
	cmd := NewCommitCommand()

	flag := cmd.Flags().Lookup("commit-nested")
	if flag == nil {
		t.Fatal("--commit-nested flag not registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("--commit-nested default = %q, want %q", flag.DefValue, "false")
	}
}
