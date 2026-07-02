package chain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Valid(t *testing.T) {
	root := t.TempDir()
	// Create festival directories matching the naming convention
	require.NoError(t, os.MkdirAll(filepath.Join(root, "hedera-foundation-HF0001"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "chain-agents-CA0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "hf", ID: "HF0001", Name: "hedera-foundation"},
			{Ref: "ca", ID: "CA0001", Name: "chain-agents"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{root})
	require.NoError(t, err)
	assert.Len(t, resolved, 2)

	hf := resolved["hf"]
	assert.Equal(t, "HF0001", hf.Node.ID)
	assert.Equal(t, filepath.Join(root, "hedera-foundation-HF0001"), hf.Path)

	ca := resolved["ca"]
	assert.Equal(t, "CA0001", ca.Node.ID)
	assert.Equal(t, filepath.Join(root, "chain-agents-CA0001"), ca.Path)
}

func TestResolve_MultipleSearchDirs(t *testing.T) {
	active := t.TempDir()
	planning := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(active, "alpha-A0001"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(planning, "beta-B0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "b", ID: "B0001", Name: "beta"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{active, planning})
	require.NoError(t, err)
	assert.Len(t, resolved, 2)
	assert.Contains(t, resolved["a"].Path, "alpha-A0001")
	assert.Contains(t, resolved["b"].Path, "beta-B0001")
}

func TestResolve_FestivalNotFound(t *testing.T) {
	root := t.TempDir()

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "x", ID: "X0001", Name: "missing"},
		},
	}

	_, err := Resolve(context.Background(), c, []string{root})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolve_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	_, err := Resolve(ctx, c, []string{"/tmp"})
	assert.Error(t, err)
}

func TestResolve_ExactIDMatch(t *testing.T) {
	root := t.TempDir()
	// Directory named exactly as the ID (no name prefix)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "A0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	resolved, err := Resolve(context.Background(), c, []string{root})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "A0001"), resolved["a"].Path)
}

func TestResolve_SkipsUnreadableDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "alpha-A0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
		},
	}

	// Include a nonexistent dir as first search path — should skip it
	resolved, err := Resolve(context.Background(), c, []string{"/nonexistent", root})
	require.NoError(t, err)
	assert.Contains(t, resolved["a"].Path, "alpha-A0001")
}

func TestResolve_FindsFestivalInDungeonDatedBucket(t *testing.T) {
	root := t.TempDir()
	// Working festival sits directly under its status dir.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "active", "onboarding-parity-FA0010"), 0o755))
	// Completed festival lives inside a dated dungeon bucket.
	require.NoError(t, os.MkdirAll(
		filepath.Join(root, "dungeon", "completed", "2026-06-04", "audit-remediation-FA0009"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "audit", ID: "FA0009", Name: "audit-remediation"},
			{Ref: "onboarding", ID: "FA0010", Name: "onboarding-parity"},
		},
	}

	searchDirs := []string{
		filepath.Join(root, "active"),
		filepath.Join(root, "dungeon", "completed"),
	}

	resolved, err := Resolve(context.Background(), c, searchDirs)
	require.NoError(t, err)
	assert.Equal(t,
		filepath.Join(root, "dungeon", "completed", "2026-06-04", "audit-remediation-FA0009"),
		resolved["audit"].Path)
	assert.Equal(t, filepath.Join(root, "active", "onboarding-parity-FA0010"), resolved["onboarding"].Path)
}

func TestResolve_IgnoresNonDateDungeonSubdirs(t *testing.T) {
	root := t.TempDir()
	// A non-date subdir (e.g. the dungeon chains dir) must not be descended.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dungeon", "completed", "chains"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dungeon", "completed", "chains", "decoy-FA0009"), 0o755))

	c := &Chain{Festivals: []FestivalNode{{Ref: "x", ID: "FA0009", Name: "audit"}}}

	_, err := Resolve(context.Background(), c, []string{filepath.Join(root, "dungeon", "completed")})
	assert.Error(t, err, "should not match festivals under non-date subdirectories")
}

func TestResolveAvailable_OmitsMissingRefs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "alpha-A0001"), 0o755))

	c := &Chain{
		Festivals: []FestivalNode{
			{Ref: "a", ID: "A0001", Name: "alpha"},
			{Ref: "missing", ID: "M0001", Name: "missing"},
		},
	}

	resolved, err := ResolveAvailable(context.Background(), c, []string{root})
	require.NoError(t, err)
	assert.Len(t, resolved, 1, "missing ref should be omitted, not fail the whole resolution")
	assert.Contains(t, resolved["a"].Path, "alpha-A0001")
	_, ok := resolved["missing"]
	assert.False(t, ok)
}

func TestResolve_FindsFestivalInLegacyMonthBucket(t *testing.T) {
	root := t.TempDir()
	// Legacy dungeon buckets use YYYY-MM (no day); the resolver must still descend.
	require.NoError(t, os.MkdirAll(
		filepath.Join(root, "dungeon", "completed", "2026-05", "old-work-OW0001"), 0o755))

	c := &Chain{Festivals: []FestivalNode{{Ref: "ow", ID: "OW0001", Name: "old-work"}}}

	resolved, err := Resolve(context.Background(), c, []string{filepath.Join(root, "dungeon", "completed")})
	require.NoError(t, err)
	assert.Contains(t, resolved["ow"].Path, filepath.Join("2026-05", "old-work-OW0001"))
}

func TestResolveAvailable_FindsDungeonFestival(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(
		filepath.Join(root, "dungeon", "completed", "2026-06-04", "audit-remediation-FA0009"), 0o755))

	c := &Chain{Festivals: []FestivalNode{{Ref: "audit", ID: "FA0009", Name: "audit-remediation"}}}

	resolved, err := ResolveAvailable(context.Background(), c, []string{filepath.Join(root, "dungeon", "completed")})
	require.NoError(t, err)
	assert.Contains(t, resolved["audit"].Path, filepath.Join("2026-06-04", "audit-remediation-FA0009"))
}

func TestResolveAvailable_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Chain{Festivals: []FestivalNode{{Ref: "a", ID: "A0001", Name: "alpha"}}}
	_, err := ResolveAvailable(ctx, c, []string{"/tmp"})
	assert.Error(t, err)
}

func TestIsDateBucket(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"2026-06-04", true},
		{"2026-12-31", true},
		{"2026-05", true}, // legacy YYYY-MM bucket
		{"2025-01", true}, // legacy YYYY-MM bucket
		{"2026-5", false}, // single-digit month
		{"2026", false},   // year only
		{"chains", false},
		{"2026-6-4", false},
		{"2026-06-04-extra", false},
		{"audit-remediation-FA0009", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isDateBucket(tt.name))
		})
	}
}

func TestMatchesFestivalID(t *testing.T) {
	tests := []struct {
		dirName    string
		festivalID string
		want       bool
	}{
		{"hedera-foundation-HF0001", "HF0001", true},
		{"chain-agents-CA0001", "CA0001", true},
		{"HF0001", "HF0001", true},
		{"some-other-dir", "HF0001", false},
		{"HF0001-extra", "HF0001", false},
		{"", "HF0001", false},
	}

	for _, tt := range tests {
		t.Run(tt.dirName, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesFestivalID(tt.dirName, tt.festivalID))
		})
	}
}
